package totem_test

import (
	context "context"
	"io"
	"time"

	totem "github.com/kralicky/totem"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Test", func() {
	It("should work with two different servers", func() {
		runScenarioA(func(client SToCClient, errC <-chan error) error {
			q, err := client.AskClient(context.Background(), &SRequest{
				Answer: "THERE IS AS YET INSUFFICIENT DATA FOR A MEANINGFUL ANSWER",
			})
			Expect(err).To(BeNil())
			Expect(q.Question).To(Equal("How can entropy be reversed?"))

			q, err = client.AskClient(context.Background(), &SRequest{
				Answer: "42",
			})
			Expect(err).To(BeNil())
			Expect(q.Question).To(Equal("What is the meaning of life?"))

			q, err = client.AskClient(context.Background(), &SRequest{
				Answer: "34",
			})
			Expect(err).To(BeNil())
			Expect(q.Question).To(Equal("What is the length of this string?"))

			q, err = client.AskClient(context.Background(), &SRequest{
				Answer: "unknown",
			})
			Expect(err).To(MatchError(errUnknown))

			return totem.WaitErrOrTimeout(errC, 5*time.Second)
		}, func(client CToSClient, errC <-chan error) error {
			resp, err := client.AskServer(context.Background(), &CRequest{
				Question: "How can entropy be reversed?",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Answer).To(Equal("THERE IS AS YET INSUFFICIENT DATA FOR A MEANINGFUL ANSWER"))

			resp, err = client.AskServer(context.Background(), &CRequest{
				Question: "What is the meaning of life?",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Answer).To(Equal("42"))

			resp, err = client.AskServer(context.Background(), &CRequest{
				Question: "What is the length of this string?",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Answer).To(Equal("34"))

			resp, err = client.AskServer(context.Background(), &CRequest{
				Question: "unknown",
			})
			Expect(err).To(MatchError(errUnknown))

			return nil
		})
	})

	It("should work with the same server on both sides", func() {
		runScenarioB(func(client CalculatorClient, errC <-chan error) error {
			resp, err := client.Add(context.Background(), &AddRequest{
				Lhs: 1,
				Rhs: 2,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Result).To(Equal(int64(3)))
			return totem.WaitErrOrTimeout(errC, 5*time.Second)
		}, func(client CalculatorClient, errC <-chan error) error {
			resp, err := client.Add(context.Background(), &AddRequest{
				Lhs: 10,
				Rhs: 20,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Result).To(Equal(int64(30)))
			return nil
		})
	})

	It("should handle the server ending the stream", func() {
		runScenarioB(func(client CalculatorClient, errC <-chan error) error {
			resp, err := client.Add(context.Background(), &AddRequest{
				Lhs: 1,
				Rhs: 2,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Result).To(Equal(int64(3)))
			time.Sleep(100 * time.Millisecond)
			return nil
		}, func(client CalculatorClient, errC <-chan error) error {
			defer GinkgoRecover()
			resp, err := client.Add(context.Background(), &AddRequest{
				Lhs: 10,
				Rhs: 20,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Result).To(Equal(int64(30)))
			Eventually(errC).Should(Receive(Equal(io.EOF)))
			return nil
		})
	})
})
