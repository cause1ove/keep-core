package signing

import (
	"fmt"
	"math/big"

	tsslibcommon "github.com/bnb-chain/tss-lib/common"
	"github.com/bnb-chain/tss-lib/ecdsa/signing"
	"github.com/bnb-chain/tss-lib/tss"
	"github.com/ipfs/go-log"
	"github.com/keep-network/keep-core/pkg/crypto/ephemeral"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/tecdsa"
	"github.com/keep-network/keep-core/pkg/tecdsa/common"
	"golang.org/x/exp/slices"
)

// Member represents a signing protocol member.
type member struct {
	// Logger used to produce log messages.
	logger log.StandardLogger
	// id of this group member.
	id group.MemberIndex
	// Group to which this member belongs.
	group *group.Group
	// Validator allowing to check public key and member index against
	// group members
	membershipValidator *group.MembershipValidator
	// Identifier of the particular signing session this member is part of.
	sessionID string
	// Message that is the subject of the signing process.
	message *big.Int
	// tECDSA private key share of the member.
	privateKeyShare *tecdsa.PrivateKeyShare
	// Instance of the member identity converter.
	identityConverter *identityConverter
}

// newMember creates a new member in an initial state
func newMember(
	logger log.StandardLogger,
	memberID group.MemberIndex,
	groupSize,
	dishonestThreshold int,
	membershipValidator *group.MembershipValidator,
	sessionID string,
	message *big.Int,
	privateKeyShare *tecdsa.PrivateKeyShare,
) *member {
	return &member{
		logger:              logger,
		id:                  memberID,
		group:               group.NewGroup(dishonestThreshold, groupSize),
		membershipValidator: membershipValidator,
		sessionID:           sessionID,
		message:             message,
		privateKeyShare:     privateKeyShare,
		identityConverter:   &identityConverter{keys: privateKeyShare.Data().Ks},
	}
}

// inactiveMemberFilter returns a new instance of the inactive member filter.
func (m *member) inactiveMemberFilter() *group.InactiveMemberFilter {
	return group.NewInactiveMemberFilter(m.logger, m.id, m.group)
}

// shouldAcceptMessage indicates whether the given member should accept
// a message from the given sender.
func (m *member) shouldAcceptMessage(
	senderID group.MemberIndex,
	senderPublicKey []byte,
) bool {
	isMessageFromSelf := senderID == m.id
	isSenderValid := m.membershipValidator.IsValidMembership(
		senderID,
		senderPublicKey,
	)
	isSenderAccepted := m.group.IsOperating(senderID)

	return !isMessageFromSelf && isSenderValid && isSenderAccepted
}

// initializeEphemeralKeysGeneration performs a transition of a member state
// from the initial state to the first phase of the protocol.
func (m *member) initializeEphemeralKeysGeneration() *ephemeralKeyPairGeneratingMember {
	return &ephemeralKeyPairGeneratingMember{
		member:            m,
		ephemeralKeyPairs: make(map[group.MemberIndex]*ephemeral.KeyPair),
	}
}

// ephemeralKeyPairGeneratingMember represents one member in a signing group
// performing ephemeral key pair generation. It has a full list of `memberIDs`
// that belong to its threshold group.
type ephemeralKeyPairGeneratingMember struct {
	*member

	// Ephemeral key pairs used to create symmetric keys,
	// generated individually for each other group member.
	ephemeralKeyPairs map[group.MemberIndex]*ephemeral.KeyPair
}

// initializeSymmetricKeyGeneration performs a transition of the member state
// to the next phase. It returns a member instance ready to execute the
// next phase of the protocol.
func (ekpgm *ephemeralKeyPairGeneratingMember) initializeSymmetricKeyGeneration() *symmetricKeyGeneratingMember {
	return &symmetricKeyGeneratingMember{
		ephemeralKeyPairGeneratingMember: ekpgm,
		symmetricKeys:                    make(map[group.MemberIndex]ephemeral.SymmetricKey),
	}
}

// symmetricKeyGeneratingMember represents one member in a signing group
// performing ephemeral symmetric key generation.
type symmetricKeyGeneratingMember struct {
	*ephemeralKeyPairGeneratingMember

	// Symmetric keys used to encrypt confidential information,
	// generated individually for each other group member by ECDH'ing the
	// broadcasted ephemeral public key intended for this member and the
	// ephemeral private key generated for the other member.
	symmetricKeys map[group.MemberIndex]ephemeral.SymmetricKey
}

// markInactiveMembers takes all messages from the previous signing protocol
// execution phase and marks all member who did not send a message as IA.
func (skgm *symmetricKeyGeneratingMember) markInactiveMembers(
	ephemeralPubKeyMessages []*ephemeralPublicKeyMessage,
) {
	filter := skgm.inactiveMemberFilter()
	for _, message := range ephemeralPubKeyMessages {
		filter.MarkMemberAsActive(message.senderID)
	}

	filter.FlushInactiveMembers()
}

// initializeTssRoundOne returns a member to perform next protocol operations.
func (skgm *symmetricKeyGeneratingMember) initializeTssRoundOne() *tssRoundOneMember {
	// Set up the local TSS party using only operating members. This effectively
	// removes all excluded members who were marked as disqualified at the
	// beginning of the protocol.
	tssPartyID, groupTssPartiesIDs := common.GenerateTssPartiesIDs(
		skgm.id,
		skgm.group.OperatingMemberIDs(),
		skgm.identityConverter,
	)

	tssParameters := tss.NewParameters(
		tecdsa.Curve,
		tss.NewPeerContext(tss.SortPartyIDs(groupTssPartiesIDs)),
		tssPartyID,
		len(groupTssPartiesIDs),
		skgm.group.HonestThreshold()-1,
	)

	tssOutgoingMessagesChan := make(chan tss.Message, len(groupTssPartiesIDs))
	tssResultChan := make(chan tsslibcommon.SignatureData, 1)

	tssParty := signing.NewLocalParty(
		skgm.message,
		tssParameters,
		skgm.privateKeyShare.Data(),
		tssOutgoingMessagesChan,
		tssResultChan,
	)

	return &tssRoundOneMember{
		symmetricKeyGeneratingMember: skgm,
		tssParty:                     tssParty,
		tssParameters:                tssParameters,
		tssOutgoingMessagesChan:      tssOutgoingMessagesChan,
		tssResultChan:                tssResultChan,
	}
}

// tssRoundOneMember represents one member in a signing group performing the
// first round of the TSS keygen.
type tssRoundOneMember struct {
	*symmetricKeyGeneratingMember

	tssParty                tss.Party
	tssParameters           *tss.Parameters
	tssOutgoingMessagesChan <-chan tss.Message
	tssResultChan           <-chan tsslibcommon.SignatureData
}

// initializeTssRoundTwo returns a member to perform next protocol operations.
func (trom *tssRoundOneMember) initializeTssRoundTwo() *tssRoundTwoMember {
	return &tssRoundTwoMember{
		tssRoundOneMember: trom,
	}
}

// tssRoundTwoMember represents one member in a signing group performing the
// second round of the TSS keygen.
type tssRoundTwoMember struct {
	*tssRoundOneMember
}

// markInactiveMembers takes all messages from the previous signing protocol
// execution phase and marks all member who did not send a message as inactive.
func (trtm *tssRoundTwoMember) markInactiveMembers(
	tssRoundOneMessages []*tssRoundOneMessage,
) {
	filter := trtm.inactiveMemberFilter()
	for _, message := range tssRoundOneMessages {
		filter.MarkMemberAsActive(message.senderID)
	}

	filter.FlushInactiveMembers()
}

// initializeTssRoundThree returns a member to perform next protocol operations.
func (trtm *tssRoundTwoMember) initializeTssRoundThree() *tssRoundThreeMember {
	return &tssRoundThreeMember{
		tssRoundTwoMember: trtm,
	}
}

// tssRoundThreeMember represents one member in a signing group performing the
// third round of the TSS keygen.
type tssRoundThreeMember struct {
	*tssRoundTwoMember
}

// markInactiveMembers takes all messages from the previous signing protocol
// execution phase and marks all member who did not send a message as inactive.
func (trtm *tssRoundThreeMember) markInactiveMembers(
	tssRoundTwoMessages []*tssRoundTwoMessage,
) {
	filter := trtm.inactiveMemberFilter()
	for _, message := range tssRoundTwoMessages {
		filter.MarkMemberAsActive(message.senderID)
	}

	filter.FlushInactiveMembers()
}

// initializeTssRoundFour returns a member to perform next protocol operations.
func (trtm *tssRoundThreeMember) initializeTssRoundFour() *tssRoundFourMember {
	return &tssRoundFourMember{
		tssRoundThreeMember: trtm,
	}
}

// tssRoundFourMember represents one member in a signing group performing the
// fourth round of the TSS keygen.
type tssRoundFourMember struct {
	*tssRoundThreeMember
}

// markInactiveMembers takes all messages from the previous signing protocol
// execution phase and marks all member who did not send a message as inactive.
func (trtm *tssRoundFourMember) markInactiveMembers(
	tssRoundThreeMessages []*tssRoundThreeMessage,
) {
	filter := trtm.inactiveMemberFilter()
	for _, message := range tssRoundThreeMessages {
		filter.MarkMemberAsActive(message.senderID)
	}

	filter.FlushInactiveMembers()
}

// initializeTssRoundFive returns a member to perform next protocol operations.
func (trfm *tssRoundFourMember) initializeTssRoundFive() *tssRoundFiveMember {
	return &tssRoundFiveMember{
		tssRoundFourMember: trfm,
	}
}

// tssRoundFiveMember represents one member in a signing group performing the
// fifth round of the TSS keygen.
type tssRoundFiveMember struct {
	*tssRoundFourMember
}

// markInactiveMembers takes all messages from the previous signing protocol
// execution phase and marks all member who did not send a message as inactive.
func (trfm *tssRoundFiveMember) markInactiveMembers(
	tssRoundFourMessages []*tssRoundFourMessage,
) {
	filter := trfm.inactiveMemberFilter()
	for _, message := range tssRoundFourMessages {
		filter.MarkMemberAsActive(message.senderID)
	}

	filter.FlushInactiveMembers()
}

// initializeTssRoundSix returns a member to perform next protocol operations.
func (trfm *tssRoundFiveMember) initializeTssRoundSix() *tssRoundSixMember {
	return &tssRoundSixMember{
		tssRoundFiveMember: trfm,
	}
}

// tssRoundSixMember represents one member in a signing group performing the
// sixth round of the TSS keygen.
type tssRoundSixMember struct {
	*tssRoundFiveMember
}

// markInactiveMembers takes all messages from the previous signing protocol
// execution phase and marks all member who did not send a message as inactive.
func (trsm *tssRoundSixMember) markInactiveMembers(
	tssRoundFiveMessages []*tssRoundFiveMessage,
) {
	filter := trsm.inactiveMemberFilter()
	for _, message := range tssRoundFiveMessages {
		filter.MarkMemberAsActive(message.senderID)
	}

	filter.FlushInactiveMembers()
}

// initializeTssRoundSeven returns a member to perform next protocol operations.
func (trsm *tssRoundSixMember) initializeTssRoundSeven() *tssRoundSevenMember {
	return &tssRoundSevenMember{
		tssRoundSixMember: trsm,
	}
}

// tssRoundSevenMember represents one member in a signing group performing the
// seventh round of the TSS keygen.
type tssRoundSevenMember struct {
	*tssRoundSixMember
}

// markInactiveMembers takes all messages from the previous signing protocol
// execution phase and marks all member who did not send a message as inactive.
func (trsm *tssRoundSevenMember) markInactiveMembers(
	tssRoundSixMessages []*tssRoundSixMessage,
) {
	filter := trsm.inactiveMemberFilter()
	for _, message := range tssRoundSixMessages {
		filter.MarkMemberAsActive(message.senderID)
	}

	filter.FlushInactiveMembers()
}

// initializeTssRoundEight returns a member to perform next protocol operations.
func (trsm *tssRoundSevenMember) initializeTssRoundEight() *tssRoundEightMember {
	return &tssRoundEightMember{
		tssRoundSevenMember: trsm,
	}
}

// tssRoundEightMember represents one member in a signing group performing the
// eighth round of the TSS keygen.
type tssRoundEightMember struct {
	*tssRoundSevenMember
}

// markInactiveMembers takes all messages from the previous signing protocol
// execution phase and marks all member who did not send a message as inactive.
func (trem *tssRoundEightMember) markInactiveMembers(
	tssRoundSevenMessages []*tssRoundSevenMessage,
) {
	filter := trem.inactiveMemberFilter()
	for _, message := range tssRoundSevenMessages {
		filter.MarkMemberAsActive(message.senderID)
	}

	filter.FlushInactiveMembers()
}

// initializeTssRoundNine returns a member to perform next protocol operations.
func (trem *tssRoundEightMember) initializeTssRoundNine() *tssRoundNineMember {
	return &tssRoundNineMember{
		tssRoundEightMember: trem,
	}
}

// tssRoundNineMember represents one member in a signing group performing the
// ninth round of the TSS keygen.
type tssRoundNineMember struct {
	*tssRoundEightMember
}

// markInactiveMembers takes all messages from the previous signing protocol
// execution phase and marks all member who did not send a message as inactive.
func (trnm *tssRoundNineMember) markInactiveMembers(
	tssRoundEightMessages []*tssRoundEightMessage,
) {
	filter := trnm.inactiveMemberFilter()
	for _, message := range tssRoundEightMessages {
		filter.MarkMemberAsActive(message.senderID)
	}

	filter.FlushInactiveMembers()
}

// initializeFinalization returns a member to perform next protocol operations.
func (trnm *tssRoundNineMember) initializeFinalization() *finalizingMember {
	return &finalizingMember{
		tssRoundNineMember: trnm,
	}
}

// finalizingMember represents one member of the given group, after it
// completed the signing process.
//
// Prepares a result in the last phase of the protocol.
type finalizingMember struct {
	*tssRoundNineMember

	tssResult *tsslibcommon.SignatureData
}

// markInactiveMembers takes all messages from the previous signing protocol
// execution phase and marks all member who did not send a message as inactive.
func (fm *finalizingMember) markInactiveMembers(
	tssRoundNineMessages []*tssRoundNineMessage,
) {
	filter := fm.inactiveMemberFilter()
	for _, message := range tssRoundNineMessages {
		filter.MarkMemberAsActive(message.senderID)
	}

	filter.FlushInactiveMembers()
}

// Result is successful computation of the tECDSA signature.
func (fm *finalizingMember) Result() *Result {
	return &Result{Signature: tecdsa.NewSignature(fm.tssResult)}
}

// identityConverter implements the common.IdentityConverter for tECDSA signing.
// It does the conversion using the predefined keys list obtained from Ks
// party ID array available in TSS key share.
type identityConverter struct {
	keys []*big.Int
}

func (ic *identityConverter) MemberIndexToTssPartyID(
	memberIndex group.MemberIndex,
) *tss.PartyID {
	partyIDKey := ic.MemberIndexToTssPartyIDKey(memberIndex)

	return tss.NewPartyID(
		partyIDKey.Text(10),
		fmt.Sprintf("member-%v", memberIndex),
		partyIDKey,
	)
}

func (ic *identityConverter) MemberIndexToTssPartyIDKey(
	memberIndex group.MemberIndex,
) *big.Int {
	return ic.keys[memberIndex-1]
}

func (ic *identityConverter) TssPartyIDToMemberIndex(
	partyID *tss.PartyID,
) group.MemberIndex {
	index := slices.IndexFunc(ic.keys, func(key *big.Int) bool {
		return key.Cmp(partyID.KeyInt()) == 0
	})

	return group.MemberIndex(index + 1)
}
