package utility

import "github.com/apito-io/engine/models"

func ArrayContainsInterface(arr []interface{}, val interface{}) bool {
	for _, k := range arr {
		switch v := val.(type) {
		case string:
			if str, ok := k.(string); ok && str == v {
				return true
			}
		case int:
			if num, ok := k.(int); ok && num == v {
				return true
			}
		case float64:
			if num, ok := k.(float64); ok && num == v {
				return true
			}
		default:
			if k == val {
				return true
			}
		}
	}
	return false
}

func ArrayContains(arr []string, str string) bool {
	for _, k := range arr {
		if k == str {
			return true
		}
	}
	return false
}

// FilterUserArray filters the array by a key and removes the found item from the array
func FilterUserArray(arr *[]*models.SystemUser, key string) *models.SystemUser {
	for i, user := range *arr {
		if user.ID == key {
			// Found the item, now remove it from the array
			removedItem := user
			*arr = append((*arr)[:i], (*arr)[i+1:]...)
			return removedItem
		}
	}
	return nil
}

// FilterProjectArray filters the array by a key and removes the found item from the array
func FilterProjectArray(arr *[]*models.Project, key string) *models.Project {
	for i, user := range *arr {
		if user.ID == key {
			// Found the item, now remove it from the array
			removedItem := user
			*arr = append((*arr)[:i], (*arr)[i+1:]...)
			return removedItem
		}
	}
	return nil
}
