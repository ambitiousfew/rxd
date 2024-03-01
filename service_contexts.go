package rxd

// type ServiceContextOperation int

// const (
// 	AddContext ServiceContextOperation = iota
// 	DeleteContext
// 	GetContext
// )

// type serviceContextRequest struct {
// 	operation ServiceContextOperation
// 	service   Service
// 	context   context.Context
// 	cancel    context.CancelFunc
// 	response  chan *serviceContextResponse // Used for operations that require a response
// }

// type serviceContextResponse struct {
// 	context context.Context
// 	cancel  context.CancelFunc
// 	exists  bool
// }

// func manageServiceContexts(requests chan serviceContextRequest) {
// 	serviceContexts := make(map[Service]context.Context)
// 	serviceCancels := make(map[Service]context.CancelFunc)

// 	for request := range requests {
// 		switch request.operation {
// 		case AddContext:
// 			serviceContexts[request.service] = request.context
// 			serviceCancels[request.service] = request.cancel
// 		case DeleteContext:
// 			delete(serviceContexts, request.service)
// 			delete(serviceCancels, request.service)
// 		case GetContext:
// 			ctx, ctxExists := serviceContexts[request.service]
// 			cancel, cancelExists := serviceCancels[request.service]
// 			request.response <- &serviceContextResponse{
// 				context: ctx,
// 				cancel:  cancel,
// 				exists:  ctxExists && cancelExists,
// 			}
// 		}
// 	}
// }
