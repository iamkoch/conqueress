package store

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"testing"
)

func TestConqueressMongo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "conqueress-mongo")
}
