package zkp

import (
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/keep-network/paillier"
)

// TestZKP1CommitValues creates a commitment and checks all
// commitment values against expected ones.
//
// This is not a full roundtrip test. We test the private commitment phase
// interface to make sure if anything goes wrong in future (e.g. curve
// implementation changes), we can isolate the problem easily.
// All expected values has been manually calculated based on the [GGN16] paper.
func TestZKP1CommitValues(t *testing.T) {
	// GIVEN
	mockRandom := &mockRandReader{
		counter: big.NewInt(10),
	}
	// alpha=10
	// beta=11
	// rho=12
	// gamma=13

	params := generateTestPublicParams()

	secretKeyShare := big.NewInt(8)

	encryptedMessageShare := &paillier.Cypher{C: big.NewInt(7)}
	c1 := &paillier.Cypher{C: big.NewInt(9)}
	encryptedSecretKeyShare := &paillier.Cypher{C: big.NewInt(9)}

	r := big.NewInt(7)

	// WHEN
	zkp, err := CommitDsaPaillierSecretKeyFactorRange(c1, encryptedMessageShare, encryptedSecretKeyShare, secretKeyShare, r, params, mockRandom)
	if err != nil {
		t.Fatal(err)
	}

	// THEN

	// 1082^10 * 11^1081 mod 1168561 = 289613
	if zkp.z.Cmp(big.NewInt(289613)) != 0 {
		t.Errorf("Unexpected z\nActual: %v\nExpected: 289613", zkp.z)
	}

	// 7^10 mod 1168561 = 852048
	if zkp.v.Cmp(big.NewInt(852048)) != 0 {
		t.Errorf("Unexpected v\nActual: %v\nExpected: 852048", zkp.v)
	}

	// 20535^8 * 20919^12  mod 25651 = 7002
	if zkp.u1.Cmp(big.NewInt(7002)) != 0 {
		t.Errorf("Unexpected u1\nActual: %v\nExpected: 7002", zkp.u1)
	}

	// 20535^10 * 20919^13 mod 25651 = 1102
	if zkp.u2.Cmp(big.NewInt(1102)) != 0 {
		t.Errorf("Unexpected u2\nActual: %v\nExpected: 1102", zkp.u2)
	}

	// hash(9, 7, 8, 289613, 7002, 1101, 852048) = expectedHash
	expectedHash, _ := new(big.Int).SetString("5682122828670513769378897932249359273638734078677473798462399325423003506113", 10)
	if zkp.e.Cmp(expectedHash) != 0 {
		t.Errorf("Unexpected e\nActual: %v\nExpected: %v", zkp.e, expectedHash)
	}

	// expectedHash * 8 + 10 = expectedS1
	expectedS1, _ := new(big.Int).SetString("45456982629364110155031183457994874189109872629419790387699194603384028048914", 10)
	if zkp.s1.Cmp(expectedS1) != 0 {
		t.Errorf("Unexpected s1\nActual: %v\nExpected: %v", zkp.s1, expectedS1)
	}

	// 7^expectedHash * 11 mod 1081 = 869
	if zkp.s2.Cmp(big.NewInt(869)) != 0 {
		t.Errorf("Unexpected s2\nActual: %v\nExpected: 869", zkp.s2)
	}

	// expectedHash * 12 + 13
	expectedS3, _ := new(big.Int).SetString("68185473944046165232546775186992311283664808944129685581548791905076042073369", 10)
	if zkp.s3.Cmp(expectedS3) != 0 {
		t.Errorf("Unexpected s3\nActual: %v\nExpected: %v", zkp.s3, expectedS3)
	}
}

func TestZKP1Verification(t *testing.T) {
	//GIVEN
	params := generateTestPublicParams()

	encryptedMessageShare := big.NewInt(133808)
	c1 := big.NewInt(729674)
	encryptedSecretKeyShare := big.NewInt(688361)

	zkp := generateTestZkpPI1()

	expectedZ := big.NewInt(289613)
	actualZ := evaluateVerificationZ(encryptedSecretKeyShare, zkp.s1, zkp.s2, zkp.e, params)
	if expectedZ.Cmp(actualZ) != 0 {
		t.Errorf("Unexpected Z\nActual: %v\nExpected: %v", actualZ, expectedZ)
	}

	expectedV := big.NewInt(285526)
	actualV := evaluateVerificationV(c1, encryptedMessageShare, zkp.s1, zkp.e, params)
	if expectedV.Cmp(actualV) != 0 {
		t.Errorf("Unexpected Z\nActual: %v\nExpected: %v", actualV, expectedV)
	}

	expectedU2 := big.NewInt(1102)
	actualU2 := evaluateVerificationU2(zkp.u1, zkp.s1, zkp.s3, zkp.e, params)
	if expectedU2.Cmp(actualU2) != 0 {
		t.Errorf("Unexpected U2\nActual: %v\nExpected: %v", actualU2, expectedU2)
	}

	result := zkp.Verify(c1, encryptedMessageShare, encryptedSecretKeyShare, params)
	if !result {
		t.Errorf("Verification failed")
	}
}

func TestRoundTrip(t *testing.T) {
	// GIVEN
	message := big.NewInt(430)

	p, _ := new(big.Int).SetString("23", 10)
	q, _ := new(big.Int).SetString("47", 10)

	privateKey := paillier.CreatePrivateKey(p, q)

	params, err := GeneratePublicParameters(privateKey.N, secp256k1.S256())
	if err != nil {
		t.Fatal(err)
	}

	r, err := paillier.GetRandomNumberInMultiplicativeGroup(params.N, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	encryptedMessageShare, err := privateKey.EncryptWithR(message, r)
	if err != nil {
		t.Fatal(err)
	}

	secretKeyShare, err := rand.Int(rand.Reader, params.q)
	if err != nil {
		t.Fatalf("could not generate eta [%v]", err)
	}

	c1 := &paillier.Cypher{C: new(big.Int).Exp(encryptedMessageShare.C, secretKeyShare, params.NSquare())}

	encryptedSecretKeyShare, err := privateKey.EncryptWithR(secretKeyShare, r)
	t.Logf("encryptedSecretKeyShare: %s", encryptedSecretKeyShare.C)
	if err != nil {
		t.Fatal(err)
	}

	// WHEN
	zkp, err := CommitDsaPaillierSecretKeyFactorRange(c1, encryptedMessageShare, encryptedSecretKeyShare, secretKeyShare, r, params, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	var tests = map[string]struct {
		verify         func() bool
		expectedResult bool
	}{
		"positive validation": {
			verify: func() bool {
				return zkp.Verify(
					c1.C,
					encryptedMessageShare.C,
					encryptedSecretKeyShare.C,
					params,
				)
			},
			expectedResult: true,
		},
		"negative validation - wrong c1": {
			verify: func() bool {
				wrongC1 := big.NewInt(1411)
				return zkp.Verify(
					wrongC1,
					encryptedMessageShare.C,
					encryptedSecretKeyShare.C,
					params,
				)
			},
			expectedResult: false,
		},
		"negative validation - wrong encrypted message share": {
			verify: func() bool {
				wrongEncryptedMessageShare := big.NewInt(856)
				return zkp.Verify(
					c1.C,
					wrongEncryptedMessageShare,
					encryptedSecretKeyShare.C,
					params,
				)
			},
			expectedResult: false,
		},
		"negative validation - wrong encrypted secret key share": {
			verify: func() bool {
				wrongEncryptedSecretKeyShare := big.NewInt(798)
				return zkp.Verify(
					c1.C,
					encryptedMessageShare.C,
					wrongEncryptedSecretKeyShare,
					params,
				)
			},
			expectedResult: false,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			expectedResult := test.expectedResult
			actualResult := test.verify()
			if actualResult != expectedResult {
				t.Fatalf(
					"Expected %v from commitment validation. Got %v",
					expectedResult,
					actualResult,
				)
			}

		})
	}
}

func generateTestZkpPI1() *DsaPaillierSecretKeyFactorRangeProof {
	e, _ := new(big.Int).SetString("28665131959061509990138847422722847282246667596979352654045645230544684705784", 10)
	s1, _ := new(big.Int).SetString("315316451549676609891527321649951320104713343566772879194502097535991531763634", 10)
	s3, _ := new(big.Int).SetString("343981583508738119881666169072674167386960011163752231848547742766536216469421", 10)

	return &DsaPaillierSecretKeyFactorRangeProof{
		z:  big.NewInt(289613),
		v:  big.NewInt(285526),
		u1: big.NewInt(10797),
		u2: big.NewInt(1102),

		e: e,

		s1: s1,
		s2: big.NewInt(986),
		s3: s3,
	}
}

func generateTestPublicParams() *PublicParameters {
	return &PublicParameters{
		N:      big.NewInt(1081),  // 23 * 47
		NTilde: big.NewInt(25651), // 23 * 11

		h1: big.NewInt(20535),
		h2: big.NewInt(20919),

		q:     secp256k1.S256().Params().N,
		curve: secp256k1.S256(),
	}
}
