package entity

import "time"
import _ "github.com/mailru/easyjson/gen"

//go:generate easyjson -all
type IndexRequest struct {
	IndexRequestBody *IndexRequestBody `json:"index"`
}

type IndexRequestBody struct {
	Index string `json:"_index"`
	ID    string `json:"_id"`
}

type FieldsBody struct {
	Message       string    `json:"message"`
	Timestamp     time.Time `json:"@timestamp"`
	PodName       string    `json:"pod_name"`
	Namespace     string    `json:"namespace"`
	ContainerName string    `json:"container_name"`
	PodID         string    `json:"pod_id"`
}
