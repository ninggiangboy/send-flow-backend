package ports

type IDGenerator interface {
	New() (string, error)
}
