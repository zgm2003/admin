package captcha

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	projectredis "admin/server/internal/redis"
	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
	"github.com/redis/go-redis/v9"
	assetimages "github.com/wenlng/go-captcha-assets/resources/imagesv2"
	assettiles "github.com/wenlng/go-captcha-assets/resources/tiles"
	"github.com/wenlng/go-captcha/v2/base/option"
	"github.com/wenlng/go-captcha/v2/slide"
)

const TypeSlide = "slide"

var (
	ErrRequired         = errors.New("captcha is required")
	ErrInvalidOrExpired = errors.New("captcha is invalid or expired")
)

type GeneratedChallenge struct {
	MasterImage                         string
	TileImage                           string
	TileX, TileY, TileWidth, TileHeight int
	ImageWidth, ImageHeight             int
	AnswerX, AnswerY                    int
}

type Engine interface {
	Generate() (GeneratedChallenge, error)
}
type Secret struct {
	AnswerX int `json:"answerX"`
	AnswerY int `json:"answerY"`
}
type Store interface {
	Set(context.Context, string, Secret, time.Duration) error
	Take(context.Context, string) (*Secret, error)
}

type VerifyInput struct {
	ID   string
	X, Y int
}

type Challenge struct {
	ID          string `json:"captchaId"`
	Type        string `json:"captchaType"`
	MasterImage string `json:"masterImage"`
	TileImage   string `json:"tileImage"`
	TileX       int    `json:"tileX"`
	TileY       int    `json:"tileY"`
	TileWidth   int    `json:"tileWidth"`
	TileHeight  int    `json:"tileHeight"`
	ImageWidth  int    `json:"imageWidth"`
	ImageHeight int    `json:"imageHeight"`
	ExpiresIn   int    `json:"expiresIn"`
}

type Service struct {
	engine      Engine
	store       Store
	settings    sharedsetting.Reader
	idGenerator func() (string, error)
}
type Option func(*Service)

func WithIDGenerator(generator func() (string, error)) Option {
	return func(s *Service) {
		if generator != nil {
			s.idGenerator = generator
		}
	}
}
func NewService(engine Engine, store Store, settings sharedsetting.Reader, options ...Option) *Service {
	s := &Service{engine: engine, store: store, settings: settings, idGenerator: newID}
	for _, option := range options {
		option(s)
	}
	return s
}

func (s *Service) Generate(ctx context.Context) (Challenge, error) {
	if s == nil || s.engine == nil || s.store == nil || s.settings == nil {
		return Challenge{}, errors.New("captcha service is not configured")
	}
	ttl, err := s.settingNumber(ctx, sharedsetting.AuthCaptchaTTLKey, 1, 30)
	if err != nil {
		return Challenge{}, err
	}
	generated, err := s.engine.Generate()
	if err != nil {
		return Challenge{}, fmt.Errorf("generate captcha: %w", err)
	}
	id, err := s.idGenerator()
	if err != nil {
		return Challenge{}, fmt.Errorf("generate captcha id: %w", err)
	}
	if err := s.store.Set(ctx, id, Secret{AnswerX: generated.AnswerX, AnswerY: generated.AnswerY}, time.Duration(ttl)*time.Minute); err != nil {
		return Challenge{}, fmt.Errorf("store captcha: %w", err)
	}
	return Challenge{ID: id, Type: TypeSlide, MasterImage: generated.MasterImage, TileImage: generated.TileImage, TileX: generated.TileX, TileY: generated.TileY, TileWidth: generated.TileWidth, TileHeight: generated.TileHeight, ImageWidth: generated.ImageWidth, ImageHeight: generated.ImageHeight, ExpiresIn: ttl * 60}, nil
}

func (s *Service) Verify(ctx context.Context, input VerifyInput) error {
	if s == nil || s.store == nil || s.settings == nil {
		return errors.New("captcha service is not configured")
	}
	if strings.TrimSpace(input.ID) == "" {
		return ErrRequired
	}
	padding, err := s.settingNumber(ctx, sharedsetting.AuthCaptchaSlidePaddingKey, 0, 64)
	if err != nil {
		return err
	}
	secret, err := s.store.Take(ctx, input.ID)
	if err != nil {
		return fmt.Errorf("verify captcha: %w", err)
	}
	if secret == nil || math.Abs(float64(input.X-secret.AnswerX)) > float64(padding) || math.Abs(float64(input.Y-secret.AnswerY)) > float64(padding) {
		return ErrInvalidOrExpired
	}
	return nil
}

func (s *Service) settingNumber(ctx context.Context, key string, min, max int) (int, error) {
	row, err := s.settings.FindByKey(ctx, key)
	if err != nil {
		return 0, fmt.Errorf("read captcha policy: %w", err)
	}
	if row.IsEnabled != yesno.Yes || row.ValueType != sharedsetting.ValueTypeNumber {
		return 0, fmt.Errorf("captcha policy is invalid")
	}
	value, err := strconv.Atoi(strings.TrimSpace(row.Value))
	if err != nil || value < min || value > max {
		return 0, fmt.Errorf("captcha policy is invalid")
	}
	return value, nil
}

func newID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

type RedisStore struct {
	client redis.UniversalClient
	prefix string
}

func NewRedisStore(client *projectredis.Client, prefix string) *RedisStore {
	if client == nil {
		return nil
	}
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "captcha:slide:"
	}
	return &RedisStore{client: client.UniversalClient(), prefix: prefix}
}
func (s *RedisStore) key(id string) string { return s.prefix + strings.TrimSpace(id) }
func (s *RedisStore) Set(ctx context.Context, id string, secret Secret, ttl time.Duration) error {
	if s == nil || s.client == nil {
		return errors.New("captcha store is not configured")
	}
	payload, err := json.Marshal(secret)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, s.key(id), payload, ttl).Err()
}
func (s *RedisStore) Take(ctx context.Context, id string) (*Secret, error) {
	if s == nil || s.client == nil {
		return nil, errors.New("captcha store is not configured")
	}
	payload, err := s.client.GetDel(ctx, s.key(id)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var secret Secret
	if err := json.Unmarshal([]byte(payload), &secret); err != nil {
		return nil, err
	}
	return &secret, nil
}

type SlideEngine struct{ captcha slide.Captcha }

func NewSlideEngine() (*SlideEngine, error) {
	backgrounds, err := assetimages.GetImages()
	if err != nil {
		return nil, err
	}
	tiles, err := assettiles.GetTiles()
	if err != nil {
		return nil, err
	}
	graphs := make([]*slide.GraphImage, 0, len(tiles))
	for _, graph := range tiles {
		graphs = append(graphs, &slide.GraphImage{OverlayImage: graph.OverlayImage, ShadowImage: graph.ShadowImage, MaskImage: graph.MaskImage})
	}
	builder := slide.NewBuilder(slide.WithImageSize(option.Size{Width: 300, Height: 220}), slide.WithRangeGraphSize(option.RangeVal{Min: 42, Max: 52}))
	builder.SetResources(slide.WithBackgrounds(backgrounds), slide.WithGraphImages(graphs))
	return &SlideEngine{captcha: builder.Make()}, nil
}
func (e *SlideEngine) Generate() (GeneratedChallenge, error) {
	if e == nil || e.captcha == nil {
		return GeneratedChallenge{}, errors.New("captcha engine is not configured")
	}
	data, err := e.captcha.Generate()
	if err != nil {
		return GeneratedChallenge{}, err
	}
	master, err := data.GetMasterImage().ToBase64()
	if err != nil {
		return GeneratedChallenge{}, err
	}
	tile, err := data.GetTileImage().ToBase64()
	if err != nil {
		return GeneratedChallenge{}, err
	}
	block := data.GetData()
	size := e.captcha.GetOptions().GetImageSize()
	return GeneratedChallenge{MasterImage: master, TileImage: tile, TileX: block.DX, TileY: block.DY, TileWidth: block.Width, TileHeight: block.Height, ImageWidth: size.Width, ImageHeight: size.Height, AnswerX: block.X, AnswerY: block.Y}, nil
}
