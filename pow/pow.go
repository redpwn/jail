package pow

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"

	"github.com/ncw/gmp"
)

const version = "s"

var (
	mod = gmp.NewInt(0)
	exp = gmp.NewInt(0)
	one = gmp.NewInt(1)
	two = gmp.NewInt(2)
)

func init() {
	mod.Lsh(two, 1278)
	exp.Div(mod, gmp.NewInt(4))
	mod.Sub(mod, one)
}

type challenge struct {
	d uint32
	x *gmp.Int
}

func DecodeChallenge(v string) (*challenge, error) {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 || parts[0] != version {
		return nil, errors.New("incorrect version")
	}
	dBytes, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	if len(dBytes) > 8 {
		return nil, errors.New("difficulty too long")
	}
	// pad start with 0s to 4 bytes
	dBytes = append(make([]byte, 4-len(dBytes)), dBytes...)
	xBytes, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, err
	}
	d := binary.BigEndian.Uint32(dBytes)
	x := gmp.NewInt(0).SetBytes(xBytes)
	return &challenge{d: d, x: x}, nil
}

func GenerateChallenge(d uint32) *challenge {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return &challenge{
		x: gmp.NewInt(0).SetBytes(b),
		d: d,
	}
}

func (c *challenge) String() string {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, c.d)
	return fmt.Sprintf("%s.%s.%s", version, base64.StdEncoding.EncodeToString(b), base64.StdEncoding.EncodeToString(c.x.Bytes()))
}

func (c challenge) Solve() string {
	for i := uint32(0); i < c.d; i += 1 {
		c.x.Exp(c.x, exp, mod)
		c.x.Xor(c.x, one)
	}
	return fmt.Sprintf("%s.%s", version, base64.StdEncoding.EncodeToString(c.x.Bytes()))
}

func decodeSolution(s string) (*gmp.Int, error) {
	parts := strings.SplitN(s, ".", 2)
	if len(parts) != 2 || parts[0] != version {
		return nil, errors.New("incorrect version")
	}
	yBytes, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	return gmp.NewInt(0).SetBytes(yBytes), nil
}

func (c challenge) Check(s string) (bool, error) {
	y, err := decodeSolution(s)
	if err != nil {
		return false, fmt.Errorf("decode solution: %w", err)
	}
	for i := uint32(0); i < c.d; i += 1 {
		y.Xor(y, one)
		y.Exp(y, two, mod)
	}
	if c.x.Cmp(y) == 0 {
		return true, nil
	}
	c.x.Sub(mod, c.x)
	return c.x.Cmp(y) == 0, nil
}
