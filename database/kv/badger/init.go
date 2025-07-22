package badger

import (
	"log"

	"github.com/apito-io/engine/models"
	badger "github.com/dgraph-io/badger/v4"
)

type KVBadgerDriver struct {
	Db *badger.DB
}

func GetKVBadgerDriver(cfg *models.Config) (*KVBadgerDriver, error) {

	// It will be created if it doesn't exist.
	db, err := badger.Open(badger.DefaultOptions("./db/KV/badger"))
	if err != nil {
		log.Fatal(err)
	}
	//defer db.Close()

	return &KVBadgerDriver{Db: db}, nil
}
