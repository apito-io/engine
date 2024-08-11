package utility

import "github.com/apito-io/buffers/protobuff"

func ArrayContains(arr []string, str string) bool {
	for _, k := range arr {
		if k == str {
			return true
		}
	}
	return false
}

// FilterUserArray filters the array by a key and removes the found item from the array
func FilterUserArray(arr *[]*protobuff.SystemUser, key string) *protobuff.SystemUser {
	for i, user := range *arr {
		if user.Id == key {
			// Found the item, now remove it from the array
			removedItem := user
			*arr = append((*arr)[:i], (*arr)[i+1:]...)
			return removedItem
		}
	}
	return nil
}

// FilterProjectArray filters the array by a key and removes the found item from the array
func FilterProjectArray(arr *[]*protobuff.Project, key string) *protobuff.Project {
	for i, user := range *arr {
		if user.Id == key {
			// Found the item, now remove it from the array
			removedItem := user
			*arr = append((*arr)[:i], (*arr)[i+1:]...)
			return removedItem
		}
	}
	return nil
}
