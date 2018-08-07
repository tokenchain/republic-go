package leveldb_test

import (
	"math/rand"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/republicprotocol/republic-go/leveldb"

	"github.com/republicprotocol/republic-go/order"
	"github.com/republicprotocol/republic-go/testutils"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

var _ = Describe("MidpointPrice storage", func() {

	Context("when storing and retrieving data", func() {

		It("should be able to get the right data we store", func() {
			storer := NewMidpointPriceStorer()
			emptyPrice, err := storer.MidpointPrice(order.Tokens(0))
			Expect(err).Should(HaveOccurred())
			Expect(emptyPrice == uint64(0)).Should(BeTrue())

			price := testutils.RandMidpointPrice()
			err = storer.PutMidpointPrice(price)
			Expect(err).ShouldNot(HaveOccurred())

			storedPrice, err := storer.MidpointPrice(order.Tokens(price.TokenPairs[1]))
			Expect(err).ShouldNot(HaveOccurred())
			Expect(price.Prices[1] == storedPrice).Should(BeTrue())
		})
	})
})
