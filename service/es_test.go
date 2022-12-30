package service

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/soulgarden/logfowd/conf"
	"github.com/soulgarden/logfowd/entity"
	"github.com/valyala/fasthttp"
)

func TestCli_makeBody(t *testing.T) {
	t.Parallel()

	type fields struct {
		cfg     conf.Config
		httpCli *fasthttp.Client
		logger  *zerolog.Logger
	}

	type args struct {
		events []*entity.Event
	}

	logger := zerolog.New(os.Stdout).With().Caller().Logger()

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    bytes.Buffer
		wantErr bool
	}{
		{
			name: "test 1",
			fields: fields{
				cfg:     conf.Config{},
				httpCli: nil,
				logger:  &logger,
			},
			args: args{
				events: []*entity.Event{
					{
						Message: "testlog1",
						Time:    time.Now(),
						Meta: &entity.Meta{
							Namespace:     "test",
							PodName:       "test",
							PodID:         "klwvhKJBFEWWEWFFEW",
							ContainerName: "test",
						},
					},
					{
						Message: "testlog2",
						Time:    time.Now(),
						Meta: &entity.Meta{
							Namespace:     "test",
							PodName:       "test",
							PodID:         "klwvhKJBFEWWEWFFEW",
							ContainerName: "test",
						},
					},
					{
						Message: "testlog3",
						Time:    time.Now(),
						Meta: &entity.Meta{
							Namespace:     "test",
							PodName:       "test",
							PodID:         "klwvhKJBFEWWEWFFEW",
							ContainerName: "test",
						},
					},
				},
			},
			want:    bytes.Buffer{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := NewESCli(tt.fields.cfg, tt.fields.logger)

			_, err := s.makeBody(tt.args.events)
			if (err != nil) != tt.wantErr {
				t.Errorf("makeBody() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
		})
	}
}

func BenchmarkCli_makeBody(b *testing.B) {
	s := NewESCli(conf.Config{}, nil)

	const eventsNum = 100

	events := make([]*entity.Event, eventsNum)

	for i := 0; i < eventsNum; i++ {
		events[i] = &entity.Event{
			Message: "testlog1",
			Time:    time.Now(),
			Meta: &entity.Meta{
				Namespace:     "test",
				PodName:       "test",
				PodID:         "klwvhKJBFEWWEWFFEW",
				ContainerName: "test",
			},
		}
	}

	for i := 0; i < b.N; i++ {
		buf, err := s.makeBody(events)
		if err != nil {
			b.Error(err)

			b.FailNow()
		}

		buf.Reset()
		s.buffers.Put(buf)
	}
}
