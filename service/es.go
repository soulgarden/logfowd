package service

import (
	"bytes"
	"encoding/base64"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/mailru/easyjson"

	"github.com/rs/zerolog"
	uuid "github.com/satori/go.uuid"
	"github.com/soulgarden/logfowd/conf"
	"github.com/soulgarden/logfowd/dictionary"
	"github.com/soulgarden/logfowd/entity"
	"github.com/valyala/fasthttp"
)

type Cli struct {
	cfg           conf.Config
	httpCli       *fasthttp.Client
	buffers       sync.Pool
	fieldsBodies  sync.Pool
	indexRequests sync.Pool
	logger        *zerolog.Logger
}

func NewESCli(cfg conf.Config, logger *zerolog.Logger) *Cli {
	return &Cli{
		cfg:     cfg,
		httpCli: &fasthttp.Client{},
		buffers: sync.Pool{
			New: func() interface{} { return &bytes.Buffer{} },
		},
		fieldsBodies: sync.Pool{
			New: func() interface{} {
				return &entity.FieldsBody{}
			},
		},
		indexRequests: sync.Pool{
			New: func() interface{} { return &entity.IndexRequest{IndexRequestBody: &entity.IndexRequestBody{}} },
		},
		logger: logger,
	}
}

func (s *Cli) SendEvents(events []*entity.Event) error {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()

	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	buf, err := s.makeBody(events)
	if err != nil {
		s.logger.Err(err).Msg("make body")

		return err
	}

	req.SetBody(buf.Bytes())

	buf.Reset()
	s.buffers.Put(buf)

	req.Header.SetMethod(fasthttp.MethodPost)

	req.Header.SetContentType("application/json")

	if s.cfg.ES.UseAuth {
		req.Header.Set(
			"Authorization",
			"Basic "+base64.StdEncoding.EncodeToString([]byte(s.cfg.ES.Username+":"+s.cfg.ES.Password)),
		)
	}

	req.SetRequestURI(
		s.cfg.ES.Host + ":" + s.cfg.ES.Port + s.cfg.ES.APIPrefix + s.getIndexName() + "/_bulk",
	)

	if err := s.makeRequest(req, resp); err != nil {
		return err
	}

	return nil
}

func (s *Cli) getIndexName() string {
	return s.cfg.ES.IndexName + "-" + time.Now().Format("2006.01.02")
}

func (s *Cli) makeBody(events []*entity.Event) (*bytes.Buffer, error) {
	buf, ok := s.buffers.Get().(*bytes.Buffer)
	if !ok {
		return buf, dictionary.ErrInterfaceAssertion
	}

	var indexRequest *entity.IndexRequest

	var fieldsBody *entity.FieldsBody

	for _, event := range events {
		indexRequest, ok = s.indexRequests.Get().(*entity.IndexRequest)
		if !ok {
			return buf, dictionary.ErrInterfaceAssertion
		}

		indexRequest.IndexRequestBody.ID = uuid.NewV4().String()
		indexRequest.IndexRequestBody.Index = s.getIndexName()

		marshalled, err := easyjson.Marshal(indexRequest)
		if err != nil {
			return buf, err
		}

		buf.Write(marshalled)
		buf.Write([]byte("\n"))

		fieldsBody, ok = s.fieldsBodies.Get().(*entity.FieldsBody)
		if !ok {
			return buf, dictionary.ErrInterfaceAssertion
		}

		fieldsBody.Message = event.Message
		fieldsBody.Timestamp = event.Time
		fieldsBody.PodName = event.PodName
		fieldsBody.Namespace = event.Namespace
		fieldsBody.ContainerName = event.ContainerName
		fieldsBody.PodID = event.PodID

		marshalled, err = easyjson.Marshal(fieldsBody)
		if err != nil {
			return buf, err
		}

		buf.Write(marshalled)
		buf.Write([]byte("\n"))

		s.indexRequests.Put(indexRequest)
		s.fieldsBodies.Put(fieldsBody)
	}

	return buf, nil
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
