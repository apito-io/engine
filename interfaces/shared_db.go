package interfaces

type SharedDBInterface interface {
	Get(id string) (interface{}, error)
	Set(id string, data interface{}) error
}
