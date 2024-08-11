package utility

import (
	"errors"
	"math/rand"
	"regexp"
	"strconv"
	"time"
)

const letterBytes = "ABCDEFGHIJKLMNOPQRSTUVWXYZ01234567890"
const varBytes = "abcdefghijklmnopqrrstubwxyz"
const numberBytes = "1234567890"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

func RandomStringGenerator(n int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	// A rand.Int63() generates 63 random bits, enough for letterIdxMax letters!
	for i, cache, remain := n-1, rand.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = rand.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}

func RandomVariableGenerator(n int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	// A rand.Int63() generates 63 random bits, enough for letterIdxMax letters!
	for i, cache, remain := n-1, rand.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = rand.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(varBytes) {
			b[i] = varBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}

func RandomNumberGenerator(min, max int) string {
	rand.Seed(time.Now().Unix())
	num := rand.Intn(max-min) + min
	return strconv.Itoa(num)
}

func FilterNumber(num string) (string, error) {
	length := len(num)
	if length < 11 {
		return "", errors.New("Invalid Length of Number")
	}
	num = num[(length - 11):]
	reg := regexp.MustCompile(`(017|015|016|018|019)[0-9]{8}$`)
	num = string(reg.Find([]byte(num)))

	if num != "" {
		return num, nil
	} else {
		return "", errors.New("Invalid Subscriber Number")
	}
}
