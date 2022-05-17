package jwz

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/crypto/bn256"
	"github.com/iden3/go-schema-processor/verifiable"
	"math/big"
	"strings"
)

/*
	File proof generation. it will be deleted
*/

// r is the mod of the finite field
const r string = "21888242871839275222246405745257275088548364400416034343698204186575808495617"

// proofPairingData describes three components of zkp proof in bn256 format.
type proofPairingData struct {
	A *bn256.G1
	B *bn256.G2
	C *bn256.G1
}

// vk is the Verification Key data structure in bn256 format.
type vk struct {
	Alpha *bn256.G1
	Beta  *bn256.G2
	Gamma *bn256.G2
	Delta *bn256.G2
	IC    []*bn256.G1
}

// vkJSON is the Verification Key data structure in string format (from json).
type vkJSON struct {
	Alpha []string   `json:"vk_alpha_1"`
	Beta  [][]string `json:"vk_beta_2"`
	Gamma [][]string `json:"vk_gamma_2"`
	Delta [][]string `json:"vk_delta_2"`
	IC    [][]string `json:"IC"`
}

// VerifyProof performs a verification of zkp  based on verification key and public inputs
func VerifyProof(zkProof verifiable.ZKProof, verificationKey []byte) error {

	// 1. cast external proof data to internal model.
	p, err := parseProofData(*zkProof.Proof)
	if err != nil {
		return err
	}

	// 2. cast external verification key data to internal model.
	var vkStr vkJSON
	err = json.Unmarshal(verificationKey, &vkStr)
	if err != nil {
		return err
	}
	vkKey, err := parseVK(vkStr)
	if err != nil {
		return err
	}

	// 2. cast external public inputs data to internal model.
	pubSignals, err := stringsToArrayBigInt(zkProof.PubSignals)
	if err != nil {
		return err
	}

	return verifyGroth16(vkKey, p, pubSignals)
}

// verifyGroth16 performs the verification the Groth16 zkSNARK proofs
func verifyGroth16(vk *vk, proof proofPairingData, inputs []*big.Int) error {
	if len(inputs)+1 != len(vk.IC) {
		return fmt.Errorf("len(inputs)+1 != len(vk.IC)")
	}
	vkX := new(bn256.G1).ScalarBaseMult(big.NewInt(0))
	for i := 0; i < len(inputs); i++ {
		// check input inside field
		v, _ := new(big.Int).SetString(r, 10)
		if inputs[i].Cmp(v) != -1 {
			return fmt.Errorf("input value is not in the fields")
		}
		vkX = new(bn256.G1).Add(vkX, new(bn256.G1).ScalarMult(vk.IC[i+1], inputs[i]))
	}
	vkX = new(bn256.G1).Add(vkX, vk.IC[0])

	g1 := []*bn256.G1{proof.A, new(bn256.G1).Neg(vk.Alpha), vkX.Neg(vkX), new(bn256.G1).Neg(proof.C)}
	g2 := []*bn256.G2{proof.B, vk.Beta, vk.Gamma, vk.Delta}

	res := bn256.PairingCheck(g1, g2)
	if !res {
		return fmt.Errorf("invalid proofs")
	}
	return nil
}

func parseProofData(pr verifiable.ProofData) (proofPairingData, error) {
	var (
		p   proofPairingData
		err error
	)

	p.A, err = stringToG1(pr.A)
	if err != nil {
		return p, err
	}

	p.B, err = stringToG2(pr.B)
	if err != nil {
		return p, err
	}

	p.C, err = stringToG1(pr.C)
	if err != nil {
		return p, err
	}

	return p, err
}

func parseVK(vkStr vkJSON) (*vk, error) {
	var v vk
	var err error
	v.Alpha, err = stringToG1(vkStr.Alpha)
	if err != nil {
		return nil, err
	}

	v.Beta, err = stringToG2(vkStr.Beta)
	if err != nil {
		return nil, err
	}

	v.Gamma, err = stringToG2(vkStr.Gamma)
	if err != nil {
		return nil, err
	}

	v.Delta, err = stringToG2(vkStr.Delta)
	if err != nil {
		return nil, err
	}

	for i := 0; i < len(vkStr.IC); i++ {
		p, err := stringToG1(vkStr.IC[i])
		if err != nil {
			return nil, err
		}
		v.IC = append(v.IC, p)
	}

	return &v, nil
}
func stringsToArrayBigInt(publicInputs []string) ([]*big.Int, error) {
	p := make([]*big.Int, 0, len(publicInputs))
	for _, s := range publicInputs {
		sb, err := stringToBigInt(s)
		if err != nil {
			return nil, err
		}
		p = append(p, sb)
	}
	return p, nil
}
func stringToBigInt(s string) (*big.Int, error) {
	base := 10
	if bytes.HasPrefix([]byte(s), []byte("0x")) {
		base = 16
		s = strings.TrimPrefix(s, "0x")
	}
	n, ok := new(big.Int).SetString(s, base)
	if !ok {
		return nil, fmt.Errorf("can not parse string to *big.Int: %s", s)
	}
	return n, nil
}
func stringToG1(h []string) (*bn256.G1, error) {
	if len(h) <= 2 {
		return nil, fmt.Errorf("not enought data for stringToG1")
	}
	h = h[:2]
	hexa := false
	if len(h[0]) > 1 {
		if h[0][:2] == "0x" {
			hexa = true
		}
	}
	in := ""

	var b []byte
	var err error
	if hexa {
		for i := range h {
			in += strings.TrimPrefix(h[i], "0x")
		}
		b, err = hex.DecodeString(in)
		if err != nil {
			return nil, err
		}
	} else {
		// TODO TMP
		// TODO use stringToBytes()
		if h[0] == "1" {
			h[0] = "0"
		}
		if h[1] == "1" {
			h[1] = "0"
		}
		bi0, ok := new(big.Int).SetString(h[0], 10)
		if !ok {
			return nil, fmt.Errorf("error parsing stringToG1")
		}
		bi1, ok := new(big.Int).SetString(h[1], 10)
		if !ok {
			return nil, fmt.Errorf("error parsing stringToG1")
		}
		b0 := bi0.Bytes()
		b1 := bi1.Bytes()
		if len(b0) != 32 {
			b0 = addZPadding(b0)
		}
		if len(b1) != 32 {
			b1 = addZPadding(b1)
		}

		b = append(b, b0...)
		b = append(b, b1...)
	}
	p := new(bn256.G1)
	_, err = p.Unmarshal(b)

	return p, err
}
func stringToG2(h [][]string) (*bn256.G2, error) {
	if len(h) <= 2 {
		return nil, fmt.Errorf("not enought data for stringToG2")
	}
	h = h[:2]
	hexa := false
	if len(h[0][0]) > 1 {
		if h[0][0][:2] == "0x" {
			hexa = true
		}
	}
	in := ""
	var (
		b   []byte
		err error
	)
	if hexa {
		for i := 0; i < len(h); i++ {
			for j := 0; j < len(h[i]); j++ {
				in += strings.TrimPrefix(h[i][j], "0x")
			}
		}
		b, err = hex.DecodeString(in)
		if err != nil {
			return nil, err
		}
	} else {
		// TODO TMP
		var bH []byte
		bH, err = stringToBytes(h[0][1])
		if err != nil {
			return nil, err
		}
		b = append(b, bH...)
		bH, err = stringToBytes(h[0][0])
		if err != nil {
			return nil, err
		}
		b = append(b, bH...)
		bH, err = stringToBytes(h[1][1])
		if err != nil {
			return nil, err
		}
		b = append(b, bH...)
		bH, err = stringToBytes(h[1][0])
		if err != nil {
			return nil, err
		}
		b = append(b, bH...)
	}

	p := new(bn256.G2)
	_, err = p.Unmarshal(b)
	return p, err
}
func addZPadding(b []byte) []byte {
	var z [32]byte
	var r []byte
	r = append(r, z[len(b):]...) // add padding on the left
	r = append(r, b...)
	return r[:32]
}
func stringToBytes(s string) ([]byte, error) {
	if s == "1" {
		s = "0"
	}
	bi, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return nil, fmt.Errorf("error parsing bigint stringToBytes")
	}
	b := bi.Bytes()
	if len(b) != 32 {
		b = addZPadding(b)
	}
	return b, nil

}
