package validation

import (
	"context"
	"fmt"

	"github.com/gardener/etcd-druid/internal/webhook/util"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type Handler struct {
	client  client.Client
	config  *Config
	decoder *util.RequestDecoder // to decode the fields in the request
	logger  logr.Logger
}

// to be called in the
func NewHandler(mgr manager.Manager, config *Config) (*Handler, error) {
	return &Handler{
		client:  mgr.GetClient(),
		config:  config,
		decoder: util.NewRequestDecoder(mgr),
		logger:  mgr.GetLogger().WithName(handlerName),
	}, nil
}

func (h *Handler) Handle(ctx context.Context, req admission.Request) admission.Response {
	fmt.Println("ENTERED THE HANDLE")
	log := h.logger.WithValues(
		"namespace", req.Namespace,
		"name", req.Name,
		"operation", req.Operation,
		"userInfo", req.UserInfo,
	)

	log.Info("REQUEST RECEIVED!!!!!!!!!!!!!!!!!",
		"namespace", req.Namespace,
		"name", req.Name,
		"operation", req.Operation,
		"userInfo", req.UserInfo,
	)
	return admission.Allowed("updation of managed resources by etcd-druid is allowed")
}
