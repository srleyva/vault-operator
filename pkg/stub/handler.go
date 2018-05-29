package stub

import (
	"github.com/srleyva/vault-operator/pkg/apis/vault/v1alpha1"
	"github.com/operator-framework/operator-sdk/pkg/sdk/handler"
	"github.com/operator-framework/operator-sdk/pkg/sdk/types"
)

func NewHandler() handler.Handler {
	return &Handler{}
}

type Handler struct {
	// Fill me
}

func (h *Handler) Handle(ctx types.Context, event types.Event) error {
	switch o := event.Object.(type) {
	case *v1alpha1.Vault:
		vault := o
		// Ignore the delete event since the garbage collector will clean up all secondary resources for the CR
		// All secondary resources must have the CR set as their OwnerReference for this to be the case
		if event.Deleted {
			return nil
		}

		if err := NewVault(vault); err != nil {
			return err
		}

	case *v1alpha1.Consul:
		consul := o

		// Ignore the delete event since the garbage collector will clean up all secondary resources for the CR
		// All secondary resources must have the CR set as their OwnerReference for this to be the case
		if event.Deleted {
			return nil
		}

		if err := NewConsul(consul); err != nil {
			return err
		}
	}
	return nil
}


