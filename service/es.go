package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/rs/zerolog"
	uuid "github.com/satori/go.uuid"
	"github.com/soulgarden/logfowd/conf"
	"github.com/soulgarden/logfowd/dictionary"
	"github.com/soulgarden/logfowd/entity"
	"github.com/valyala/fasthttp"
)

type Cli struct {
	cfg     *conf.Config
	httpCli *fasthttp.Client
	logger  *zerolog.Logger
}

func NewESCli(cfg *conf.Config, logger *zerolog.Logger) *Cli {
	return &Cli{cfg: cfg, httpCli: &fasthttp.Client{}, logger: logger}
}

func (s *Cli) SendEvents(events []*entity.Event) error {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()

	var buf bytes.Buffer

	for _, event := range events {
		marshalled, err := json.Marshal(&entity.IndexRequest{
			IndexRequestBody: &entity.IndexRequestBody{
				ID: uuid.NewV4().String(),
			},
		})
		if err != nil {
			return err
		}

		buf.Write(marshalled)
		buf.Write([]byte("\n"))

		marshalled, err = json.Marshal(&entity.FieldsBody{
			Message:       event.Message,
			Timestamp:     event.Time,
			PodName:       event.PodName,
			Namespace:     event.Namespace,
			ContainerName: event.ContainerName,
			ContainerID:   event.ContainerID,
		})
		if err != nil {
			return err
		}

		buf.Write(marshalled)
		buf.Write([]byte("\n"))
	}

	buf.Write([]byte("\n"))

	req.SetBody(buf.Bytes())

	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.SetContentType("application/json")

	index := s.cfg.ES.IndexName + "-" + time.Now().Format("2006.01.02")

	req.SetRequestURI(
		s.cfg.ES.Host + ":" + s.cfg.ES.Port + "/" + index + "/_bulk",
	)

	if err := s.makeRequest(req, resp); err != nil {
		return err
	}

	return nil
}

func (s *Cli) makeRequest(req *fasthttp.Request, resp *fasthttp.Response) error {
	httpClient := &fasthttp.Client{}

	start := time.Now()

	if err := httpClient.DoTimeout(req, resp, dictionary.RequestTimeout); err != nil {
		s.logRequest(req, resp, time.Since(start), err)

		if !errors.Is(err, fasthttp.ErrDialTimeout) {
			return err
		}

		if err := httpClient.DoTimeout(req, resp, dictionary.RequestTimeout); err != nil {
			return err
		}
	}

	if resp.StatusCode() != http.StatusOK {
		s.logRequest(req, resp, time.Since(start), dictionary.ErrBadStatusCode)

		return dictionary.ErrBadStatusCode
	}

	s.logRequest(req, resp, time.Since(start), nil)

	return nil
}

func (s *Cli) logRequest(
	req *fasthttp.Request,
	resp *fasthttp.Response,
	duration time.Duration,
	err error,
) {
	event := s.logger.
		Err(err).
		Str("request headers", req.Header.String()).
		Int("response code", resp.StatusCode()).
		Dur("duration", duration)

	if err != nil {
		event.Bytes("response body", resp.Body())
	}

	event.Msg("request")
}
