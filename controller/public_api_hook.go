package controller

// AfterPublicAPI is nil in the public engine. A cloud build may set it.
// The host is the request host. This function does not open a connection.
var AfterPublicAPI func(host, projectID, userID, protocol string)

func notePublicAPI(host, projectID, userID, protocol string) {
	if AfterPublicAPI == nil {
		return
	}
	AfterPublicAPI(host, projectID, userID, protocol)
}
