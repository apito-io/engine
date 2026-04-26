package migration

import (
	"fmt"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"golang.org/x/crypto/bcrypt"
)

func GetProjectInfo() *models.Project {
	/*envMap, err := godotenv.Read(configFile)
	if err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}*/

	return &models.Project{
		XKey:        "fitness_app_jh478",
		ID:          "fitness_app_jh478",
		Name:        "Fitness App",
		Description: "A Fitness Tracker App",
	}
}

func GetMigrationUserData() []*models.SystemUser {

	hash, err := bcrypt.GenerateFromPassword([]byte("#ApitoRocks#"), bcrypt.DefaultCost)
	if err != nil {
		fmt.Println(err.Error())
	}

	return []*models.SystemUser{
		{
			ID:               utility.NewID(),
			FirstName:        "System Admin",
			Email:            "admin@apito.io",
			Username:         "admin",
			Secret:           string(hash),
			CurrentProjectID: "fitness_app_jh478",
		},
	}
}
