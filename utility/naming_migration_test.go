package utility

import (
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
)

func TestComputeNamingV2ModelRenamePairs_AlreadyV2(t *testing.T) {
	p := &models.Project{
		Schema: &models.ProjectSchema{
			NamingSchemaVersion: NamingSchemaVersionV2,
			Models: []*models.ModelType{
				{Name: "food_item"},
			},
		},
	}
	pairs, err := ComputeNamingV2ModelRenamePairs(p)
	require.NoError(t, err)
	require.Nil(t, pairs)
}

func TestComputeNamingV2ModelRenamePairs_Renames(t *testing.T) {
	p := &models.Project{
		Schema: &models.ProjectSchema{
			NamingSchemaVersion: NamingSchemaVersionLegacy,
			Models: []*models.ModelType{
				{Name: "FoodItem"},
				{Name: "blog_post"},
			},
		},
	}
	pairs, err := ComputeNamingV2ModelRenamePairs(p)
	require.NoError(t, err)
	require.Len(t, pairs, 1)
	require.Equal(t, "FoodItem", pairs[0].Old)
	require.Equal(t, "food_item", pairs[0].New)
}
