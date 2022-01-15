package service

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/soulgarden/logfowd/conf"
	"github.com/soulgarden/logfowd/entity"
)

func TestState_SaveState(t *testing.T) {
	t.Parallel()

	rootDir := os.Getenv("ROOT_DIR")
	if rootDir == "" {
		rootDir = ".."
	}

	type fields struct {
		cfg    *conf.Config
		logger *zerolog.Logger
	}

	type args struct {
		state *entity.State
	}

	logger := zerolog.New(os.Stdout).With().Caller().Logger()

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "1",
			fields: fields{
				cfg: &conf.Config{
					Env:       "test",
					DebugMode: false,
					ES:        nil,
					StatePath: rootDir + "/storage/test1.json",
					LogsPath:  nil,
				},
				logger: &logger,
			},
			args: args{
				state: &entity.State{Files: map[string]*entity.File{
					"test": {
						Size:   12312312312312312,
						Offset: 235423423422342,
						Path:   "/var/log/pods/w12e1dewdqw/wdwefwfewf.log",
						Meta: &entity.Meta{
							Namespace:     "test",
							PodName:       "test",
							PodID:         "dsfewfwefwefw",
							ContainerName: "test",
						},
					},
				}},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := NewState(tt.fields.cfg, tt.fields.logger)
			if err := s.Open(); (err != nil) != tt.wantErr {
				t.Errorf("SaveState() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err := s.SaveState(tt.args.state); (err != nil) != tt.wantErr {
				t.Errorf("SaveState() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err := s.Close(); (err != nil) != tt.wantErr {
				t.Errorf("SaveState() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func BenchmarkState_SaveState(b *testing.B) {
	b.SetParallelism(1)

	logger := zerolog.New(os.Stdout).With().Caller().Logger()

	s := NewState(
		&conf.Config{
			Env:       "test",
			DebugMode: false,
			ES:        nil,
			StatePath: "../storage/bench1.json",
			LogsPath:  nil,
		},
		&logger,
	)

	if err := s.Open(); err != nil {
		b.Error(err)

		b.FailNow()
	}

	defer s.Close()

	state := &entity.State{Files: map[string]*entity.File{
		"test1": {
			Size:   12312312312312312,
			Offset: 235423423422342,
			Path:   "/var/log/pods/w12e1dewdqw/wdwefwfewf.log",
			Meta: &entity.Meta{
				Namespace:     "test",
				PodName:       "test",
				PodID:         "dsfewfwefwefw",
				ContainerName: "test",
			},
		},
		"test2": {
			Size:   12312312312312312,
			Offset: 235423423422342,
			Path:   "/var/log/pods/w12e1dewdqw/wdwefwfewf.log",
			Meta: &entity.Meta{
				Namespace:     "test",
				PodName:       "test",
				PodID:         "dsfewfwefwefw",
				ContainerName: "test",
			},
		},
		"test3": {
			Size:   12312312312312312,
			Offset: 235423423422342,
			Path:   "/var/log/pods/w12e1dewdqw/wdwefwfewf.log",
			Meta: &entity.Meta{
				Namespace:     "test",
				PodName:       "test",
				PodID:         "dsfewfwefwefw",
				ContainerName: "test",
			},
		},
	}}

	for i := 0; i < b.N; i++ {
		err := s.SaveState(state)
		if err != nil {
			b.Error(err)

			b.FailNow()
		}
	}
}
