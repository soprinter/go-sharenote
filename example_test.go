package sharenote_test

import (
	"fmt"
	"math/big"
	"time"

	sharenote "github.com/soprinter/go-sharenote"
)

func ExampleParseLabel() {
	n, _ := sharenote.ParseLabel("34.10Z")
	fmt.Println(n.Label(), n.Z(), n.Cents())
	// Output: 34Z10 34 10
}

func ExampleSum() {
	// Notes aggregate in linear difficulty space, not by label: two 29Z50
	// notes double the difficulty, adding exactly one whole Z.
	total, _ := sharenote.Sum(
		sharenote.MustParseLabel("29Z50"),
		sharenote.MustParseLabel("29Z50"),
	)
	fmt.Println(total.Label())
	// Output: 30Z50
}

func ExampleNote_Scale() {
	// Scaling multiplies the continuous difficulty: 10Z00 x 8 = 2^10 * 8 = 2^13.
	n, _ := sharenote.MustParseLabel("10Z00").Scale(8)
	fmt.Println(n.Label())
	// Output: 13Z00
}

func ExampleWork() {
	// A pool sums each address's shares exactly and slices the block reward by
	// the proportional law, with no float64 drift.
	alpha := new(sharenote.Work).Add(sharenote.MustParseLabel("30Z00"))
	beta := new(sharenote.Work).Add(sharenote.MustParseLabel("30Z00"))

	var pool sharenote.Work
	pool.AddRat(alpha.Rat()).AddRat(beta.Rat())

	reward := big.NewInt(1_000_000)
	payout, _ := alpha.Payout(&pool, reward)
	fmt.Println(payout)
	// Output: 500000
}

func ExampleFromHashrate() {
	// The hardest note a 1 TH/s rig can expect to mint every 5 seconds.
	n, _ := sharenote.FromHashrate(1e12, 5*time.Second)
	fmt.Println(n.Label())
	// Output: 42Z18
}

func ExampleFormatHashrate() {
	n := sharenote.MustParseLabel("20Z00")
	hps, _ := n.RequiredHashrate(1 * time.Second)
	fmt.Println(sharenote.FormatHashrate(hps))
	// Output: 1.05 MH/s
}
