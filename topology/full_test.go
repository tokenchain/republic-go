package topology

import (
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/republicprotocol/go-x"
)

var _ = Describe("Fully connected mesh topologies", func() {
	var nodes []*x.Node
	var err error

	BeforeEach(func() {
		μ.Lock()
		defer μ.Unlock()

		// Initialize all nodes.
		nodes, err = generateNodes(numberOfNodes)
		Ω(err).ShouldNot(HaveOccurred())

		// Start serving from all nodes.
		for _, n := range nodes {
			go func(node *x.Node) {
				defer GinkgoRecover()
				Ω(node.Serve()).ShouldNot(HaveOccurred())
			}(n)
		}
		time.Sleep(startTimeDelay)

		// Connect all nodes to each other concurrently.
		var wg sync.WaitGroup
		wg.Add(numberOfNodes)
		for i := 0; i < numberOfNodes; i++ {
			go func(i int) {
				defer GinkgoRecover()
				defer wg.Done()

				for j := 0; j < numberOfNodes; j++ {
					if i == j {
						continue
					}
					_, err = nodes[i].RPCPing(nodes[j].MultiAddress)
					Ω(err).ShouldNot(HaveOccurred())
				}

			}(i)
		}
		wg.Wait()
	})

	AfterEach(func() {
		// Close all nodes
		for _, n := range nodes {
			go func(node *x.Node) {
				node.Stop()
			}(n)
		}

	})

	Context("when pinging", func() {
		It("should update their DHTs", func() {
			for _, node := range nodes {
				Ω(len(node.DHT.MultiAddresses())).Should(Equal(numberOfNodes - 1))
			}
		})
		Specify("The sum of pings of all node's delegate should equal to (n-1)*n", func() {
			sum := 0
			for _, node := range nodes {
				sum += node.Delegate.(*MockDelegate).PingCount
			}

			Ω(sum).Should(Equal(numberOfNodes * (numberOfNodes - 1)))
		})
	})

	Context("Sending order fragment", func() {
		It("should be able to send and receive order fragment", func() {
			err = sendMessages(nodes)
			Ω(err).ShouldNot(HaveOccurred())
			sum := 0
			for _, node := range nodes {
				sum += node.Delegate.(*MockDelegate).FragmentCount
			}
			Ω(sum).Should(Equal(numberOfMessages))
		})
	})
})
