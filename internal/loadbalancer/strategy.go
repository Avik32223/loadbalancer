package loadbalancer

type Strategy interface {
	Next([]*BackendServer) *BackendServer
}
