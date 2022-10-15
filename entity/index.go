package entity

import "time"

type IndexRequest struct {
	IndexRequestBody *IndexRequestBody `json:"index"`
}

type IndexRequestBody struct {
	ID string `json:"_id"`
}

type FieldsBody struct {
	Message       string    `json:"message"`
	Timestamp     time.Time `json:"@timestamp"`
	PodName       string    `json:"pod_name"`
	Namespace     string    `json:"namespace"`
	ContainerName string    `json:"container_name"`
	PodID         string    `json:"pod_id"`
}
