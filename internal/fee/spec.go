package fee

import (
	"github.com/vultisig/verifier/plugin"
)

type Spec struct {
	plugin.Unimplemented
}

func NewSpec() *Spec {
	return &Spec{}
}
