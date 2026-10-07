package common

import "github.com/google/uuid"

const BaseUrl = "https://api.github.com"

func GenerateUUID() string {
	id := uuid.New()
	return id.String()
}

func GenerateRequestID() string {
	return "req" + "_" + GenerateUUID()
}
