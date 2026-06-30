/*
 * Copyright 2025 The Go-Spring Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package injecting

import (
	"context"
	"net/http"
	"reflect"
	"runtime"
	"testing"
	"time"

	"go-spring.org/spring/gs/internal/gs"
	"go-spring.org/spring/gs/internal/gs_arg"
	"go-spring.org/spring/gs/internal/gs_bean"
	"go-spring.org/spring/gs/internal/gs_cond"
	"go-spring.org/spring/gs/internal/gs_dync"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/testing/assert"
)

type Logger interface {
	Print(msg string)
}

type CtxLogger interface {
	CtxPrint(ctx context.Context, msg string)
}

type SimpleLogger struct{}

func (l SimpleLogger) Print(msg string) {}

type ZeroLogger struct {
	File string
}

func NewZeroLogger(file string) *ZeroLogger {
	return &ZeroLogger{File: file}
}

func (l *ZeroLogger) Print(msg string) {}

func (l *ZeroLogger) CtxPrint(ctx context.Context, msg string) {}

type BizLogger struct{}

func (l *BizLogger) Print(msg string) {}

type Filter interface {
	Do(ctx context.Context)
}

type FilterImpl struct{}

func (f *FilterImpl) Do(ctx context.Context) {}

type ReqFilter struct{}

func (f *ReqFilter) Do(ctx context.Context, req *http.Request) {}

type Controller struct {
	Loggers []Logger `inject:"biz,*,sys"`
	Service *Service `autowire:""`
	Filters []Filter `inject:"?"`
}

type InnerService struct {
	Filter Filter `autowire:"my_filter,lazy"`
}

type OneService struct {
	Repository *Repository `inject:""`
}

type ServiceConfig struct {
	Int int    `value:"${config.int}"`
	Str string `value:"${config.str}"`
}

type Service struct {
	InnerService
	ServiceConfig `value:"${service}"`

	OneService *OneService          `autowire:""`
	Filters    []Filter             `autowire:"my_filter?,*?"`
	Loggers    map[string]CtxLogger `inject:"*,sys?"`
	Repository *Repository          `inject:""`
	Status     int
}

func (s *Service) Destroy() {
	s.Status = 0
}

type Repository struct {
	Addr gs_dync.Value[string] `value:"${addr:=127.0.0.1:5050}"`
}

func (r *Repository) GetAddr() string {
	return r.Addr.Value()
}

type Server struct {
	addr string
	arg  ServerArg
}

type ServerArg struct {
	connTimeout  int
	readTimeout  int
	writeTimeout int
}

type ServerOption func(arg *ServerArg)

func SetConnTimeout(connTimeout int) ServerOption {
	return func(arg *ServerArg) {
		arg.connTimeout = connTimeout
	}
}

func SetReadTimeout(readTimeout int) ServerOption {
	return func(arg *ServerArg) {
		arg.readTimeout = readTimeout
	}
}

func SetWriteTimeout(writeTimeout int) ServerOption {
	return func(arg *ServerArg) {
		arg.writeTimeout = writeTimeout
	}
}

func NewServer(addr string, opts ...ServerOption) *Server {
	var arg ServerArg
	for _, opt := range opts {
		opt(&arg)
	}
	return &Server{addr: addr, arg: arg}
}

type ChildBean struct {
	Value int
}

func (b *ChildBean) Echo() {}

type TestBean struct {
	Value int
}

func (b *TestBean) NewChild() *ChildBean {
	return &ChildBean{b.Value}
}

func (b *TestBean) NewChildV2() (*ChildBean, error) {
	return &ChildBean{b.Value}, nil
}

func objectBean(i any) *gs_bean.BeanDefinition {
	return gs_bean.NewBean(reflect.ValueOf(i))
}

func provideBean(ctor any, args ...gs.Arg) *gs_bean.BeanDefinition {
	return gs_bean.NewBean(ctor, args...)
}

func extractBeans(beans []*gs_bean.BeanDefinition) (_, _ []*gs_bean.BeanDefinition) {
	return beans, beans
}

type LazyA struct {
	LazyB *LazyB `autowire:"b,lazy"`
}

type LazyB struct {
	// nolint
	dummy int `value:"${dummy:=9}"`
}

func TestInjecting(t *testing.T) {

	t.Run("lazy error - missing bean", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{})))
		beans := []*gs_bean.BeanDefinition{
			objectBean(&LazyA{}),
			objectBean(&LazyB{}),
		}
		err := r.Refresh(extractBeans(beans))
		assert.Error(t, err).Matches("cannot find bean")
	})

	t.Run("lazy success", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{})))
		a := &LazyA{}
		beans := []*gs_bean.BeanDefinition{
			objectBean(a),
			objectBean(&LazyB{}).Name("b"),
		}
		err := r.Refresh(extractBeans(beans))
		assert.That(t, err).Nil()
		assert.That(t, a.LazyB).NotNil()
	})

	t.Run("success", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"spring": map[string]any{
				"force-autowire-is-nullable": true,
			},
			"logger": map[string]any{
				"biz": map[string]any{
					"file": "biz.log",
				},
			},
			"server": map[string]any{
				"enable": map[string]any{
					"write-timeout": true,
				},
			},
			"service": map[string]any{
				"config": map[string]any{
					"int": 100,
					"str": "hello",
				},
			},
		})))

		myFilter := &FilterImpl{}

		c := &Controller{}
		s := &struct {
			Server  *Server  `inject:""`
			Service *Service `autowire:""`
		}{}

		beans := []*gs_bean.BeanDefinition{
			objectBean(s),
			objectBean(myFilter).Name("my_filter").Export(gs.As[Filter]()),
			objectBean(&ReqFilter{}).Name("my_filter"),
			objectBean(&Repository{}),
			objectBean(c).DependsOn(
				gs.BeanIDFor[*Service](),
			),
			objectBean(&Service{}).DestroyMethod("Destroy").Init(func(s *Service) {
				s.Status = 1
			}),
			provideBean(func() OneService { return OneService{} }),
			objectBean(&SimpleLogger{}).Name("rpc"),
			provideBean(func() Logger { return SimpleLogger{} }).Name("sys"),
			provideBean(NewZeroLogger, gs_arg.Tag("${logger.biz.file}")).
				Export(gs.As[Logger](), gs.As[CtxLogger]()).
				Name("biz"),
			objectBean(&BizLogger{}).Name("biz"),
			provideBean(
				NewServer,
				gs_arg.Value("127.0.0.1:9090"),
				gs_arg.Bind(SetReadTimeout, gs_arg.Value(50)).Condition(
					gs_cond.OnProperty("server.enable.read-timeout"),
				),
				gs_arg.Bind(SetWriteTimeout, gs_arg.Value(100)).Condition(
					gs_cond.OnProperty("server.enable.write-timeout").HavingValue("true"),
				),
				gs_arg.Bind(SetConnTimeout, gs_arg.Value(100)).Condition(
					gs_cond.OnBean[Logger]("biz"),
				),
			),
		}

		{
			p := objectBean(&TestBean{Value: 100})
			c := provideBean((*TestBean).NewChild, p)
			beans = append(beans, p, c)
		}

		err := r.Refresh(extractBeans(beans))
		assert.That(t, err).Nil()

		time.Sleep(time.Millisecond * 50)

		assert.That(t, len(c.Loggers)).Equal(2)
		assert.That(t, s.Service.Status).Equal(1)
		assert.That(t, s.Service.Filter).Equal(myFilter)
		assert.That(t, s.Service.Int).Equal(100)
		assert.That(t, s.Service.Str).Equal("hello")
		assert.That(t, s.Server.addr).Equal("127.0.0.1:9090")
		assert.That(t, s.Server.arg.connTimeout).Equal(100)
		assert.That(t, s.Server.arg.readTimeout).Equal(0)
		assert.That(t, s.Server.arg.writeTimeout).Equal(100)

		err = r.RefreshProperties(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"spring": map[string]any{
				"force-autowire-is-nullable": true,
			},
			"logger": map[string]any{
				"biz": map[string]any{
					"file": "biz.log",
				},
			},
			"server": map[string]any{
				"enable": map[string]any{
					"write-timeout": true,
				},
			},
			"service": map[string]any{
				"config": map[string]any{
					"int": 100,
					"str": "hello",
				},
			},
			"addr": "0.0.0.0:5050",
		})))
		assert.That(t, err).Nil()

		assert.That(t, s.Service.Repository.Addr.Value()).Equal("0.0.0.0:5050")

		r.Close()

		assert.That(t, s.Service.Status).Equal(0)
	})

	t.Run("wire error - primitive type", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		beans := []*gs_bean.BeanDefinition{
			objectBean(new(struct {
				A int `autowire:""`
			})),
		}
		err := r.Refresh(extractBeans(beans))
		assert.Error(t, err).Matches("int is not a valid injection target type")
	})

	t.Run("wire error - ambiguous bean for single value", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		beans := []*gs_bean.BeanDefinition{
			objectBean(new(struct {
				Logger Logger `autowire:""`
			})),
			objectBean(&SimpleLogger{}).Name("a").Export(gs.As[Logger]()),
			objectBean(&SimpleLogger{}).Name("b").Export(gs.As[Logger]()),
		}
		err := r.Refresh(extractBeans(beans))
		assert.Error(t, err).Matches("found 2 beans")
	})

	t.Run("wire error - slice", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		beans := []*gs_bean.BeanDefinition{
			objectBean(new(struct {
				A []int `autowire:""`
			})),
		}
		err := r.Refresh(extractBeans(beans))
		assert.Error(t, err).Matches("\\[]int is not a valid injection target type")
	})

	t.Run("wire error - invalid collection pattern", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		beans := []*gs_bean.BeanDefinition{
			objectBean(new(struct {
				Loggers []Logger `autowire:"*,*"`
			})),
		}
		err := r.Refresh(extractBeans(beans))
		assert.Error(t, err).Matches("more than one \\* in collection")
	})

	t.Run("wire error - ambiguous bean for collection", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		beans := []*gs_bean.BeanDefinition{
			objectBean(new(struct {
				Loggers []Logger `autowire:"biz,*"`
			})),
			objectBean(&ZeroLogger{}).Name("biz").Export(gs.As[Logger]()),
			objectBean(&SimpleLogger{}).Name("biz").Export(gs.As[Logger]()),
		}
		err := r.Refresh(extractBeans(beans))
		assert.Error(t, err).Matches("found 2 beans")
	})

	t.Run("wire error - duplicate bean for collection", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		beans := []*gs_bean.BeanDefinition{
			objectBean(new(struct {
				Loggers []Logger `autowire:"biz,biz"`
			})),
			objectBean(&ZeroLogger{}).Name("biz").Export(gs.As[Logger]()),
		}
		err := r.Refresh(extractBeans(beans))
		assert.Error(t, err).Matches("duplicate bean")
	})

	t.Run("wire error - no matching beans", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		beans := []*gs_bean.BeanDefinition{
			objectBean(new(struct {
				Loggers []Logger `autowire:""`
			})),
		}
		err := r.Refresh(extractBeans(beans))
		assert.Error(t, err).Matches("no beans collected")
	})

	t.Run("wire error - bean not found", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		beans := []*gs_bean.BeanDefinition{
			objectBean(new(struct {
				Loggers []Logger `autowire:"sys"`
			})),
		}
		err := r.Refresh(extractBeans(beans))
		assert.Error(t, err).Matches("cannot find bean")
	})

	t.Run("wire error - init failure", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		beans := []*gs_bean.BeanDefinition{
			objectBean(new(struct {
				Loggers []Logger `autowire:"sys"`
			})),
			objectBean(&SimpleLogger{}).Init(func(*SimpleLogger) error {
				return errutil.Explain(nil, "init error")
			}).Export(gs.As[Logger]()).Name("sys"),
		}
		err := r.Refresh(extractBeans(beans))
		assert.Error(t, err).Matches("init error")
	})

	t.Run("wire error - invalid tag", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		beans := []*gs_bean.BeanDefinition{
			objectBean(new(struct {
				Loggers []Logger `autowire:"${"`
			})),
		}
		err := r.Refresh(extractBeans(beans))
		assert.Error(t, err).Matches("invalid syntax: unmatched braces in .*")
	})

	t.Run("wire error - invalid collection tag", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		beans := []*gs_bean.BeanDefinition{
			objectBean(new(struct {
				Loggers []Logger `autowire:"*?,${"`
			})),
		}
		err := r.Refresh(extractBeans(beans))
		assert.Error(t, err).Matches("invalid syntax: unmatched braces in .*")
	})

	t.Run("wire error - collection with no matches", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		s := new(struct {
			Loggers [3]Logger `autowire:"*?"`
		})
		beans := []*gs_bean.BeanDefinition{
			objectBean(s),
			objectBean(&SimpleLogger{}).Export(gs.As[Logger]()),
		}
		err := r.Refresh(extractBeans(beans))
		assert.That(t, err).Nil()
		assert.That(t, s.Loggers).Equal([3]Logger{nil, nil, nil})
	})

	t.Run("wire error - missing property", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		beans := []*gs_bean.BeanDefinition{
			objectBean(&SimpleLogger{}).DependsOn(
				gs.BeanIDFor[*ZeroLogger](),
			),
			provideBean(NewZeroLogger),
		}
		err := r.Refresh(extractBeans(beans))
		assert.Error(t, err).Matches(`property "" does not exist`)
	})

	t.Run("wire error - missing required dependencies", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"spring": map[string]any{
				"force-autowire-is-nullable": true,
			},
		})))
		s := struct {
			Logger *ZeroLogger `inject:""`
		}{}
		beans := []*gs_bean.BeanDefinition{
			objectBean(&s),
			provideBean(NewZeroLogger),
		}
		err := r.Refresh(extractBeans(beans))
		assert.That(t, err).Nil()
		assert.That(t, s.Logger).Equal((*ZeroLogger)(nil))
	})

	t.Run("wire error - provider returning error - 1", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		s := struct {
			Logger *ZeroLogger `inject:""`
		}{}
		beans := []*gs_bean.BeanDefinition{
			objectBean(&s),
			provideBean(func() (*ZeroLogger, error) {
				return nil, errutil.Explain(nil, "init error")
			}),
		}
		err := r.Refresh(extractBeans(beans))
		assert.Error(t, err).Matches("init error")
	})

	t.Run("wire error - provider returning error - 2", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"spring": map[string]any{
				"force-autowire-is-nullable": true,
			},
		})))
		s := struct {
			Logger *ZeroLogger `inject:""`
		}{}
		beans := []*gs_bean.BeanDefinition{
			objectBean(&s),
			provideBean(func() (*ZeroLogger, error) {
				return nil, errutil.Explain(nil, "init error")
			}),
		}
		err := r.Refresh(extractBeans(beans))
		assert.That(t, err).Nil()
		assert.That(t, s.Logger).Equal((*ZeroLogger)(nil))
	})

	t.Run("wire error - provider returning error - 3", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		beans := []*gs_bean.BeanDefinition{
			provideBean(func() (*ZeroLogger, error) {
				return nil, nil
			}),
		}
		err := r.Refresh(extractBeans(beans))
		assert.Error(t, err).Matches("wire .* returned nil")
	})

	t.Run("wire error - primitive type", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		err := r.Refresh(extractBeans(nil))
		assert.That(t, err).Nil()
	})

	t.Run("wire error - malformed tag", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		beans := []*gs_bean.BeanDefinition{
			objectBean(new(struct {
				Int int `value:"int"`
			})),
		}
		err := r.Refresh(extractBeans(beans))
		assert.Error(t, err).Matches("invalid syntax tag 'int'")
	})

	t.Run("wire error - struct - missing properties", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		beans := []*gs_bean.BeanDefinition{
			objectBean(new(struct {
				ServiceConfig
			})),
		}
		err := r.Refresh(extractBeans(beans))
		assert.Error(t, err).Matches("property \"config.int\" does not exist")
	})

	t.Run("wire error - struct - missing prefixed properties", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		beans := []*gs_bean.BeanDefinition{
			objectBean(new(struct {
				ServiceConfig `value:"${svr}"`
			})),
		}
		err := r.Refresh(extractBeans(beans))
		assert.Error(t, err).Matches("property \"svr.config.int\" does not exist")
	})

	t.Run("wire error - embedded pointer value tag", func(t *testing.T) {
		type Config struct {
			Value int `value:"${value:=1}"`
		}
		r := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		beans := []*gs_bean.BeanDefinition{
			objectBean(&struct {
				*Config `value:"${config}"`
			}{}),
		}
		err := r.Refresh(extractBeans(beans))
		assert.Error(t, err).Matches("target should be a value type")
	})

	t.Run("wire error - destruction failure", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		beans := []*gs_bean.BeanDefinition{
			objectBean(&SimpleLogger{}).Destroy(func(l *SimpleLogger) error {
				return errutil.Explain(nil, "destroy error")
			}),
		}
		err := r.Refresh(extractBeans(beans))
		assert.That(t, err).Nil()
		r.Close()
	})

	t.Run("map injection", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		beans := []*gs_bean.BeanDefinition{
			objectBean(&ZeroLogger{}).Name("logger1").Export(gs.As[CtxLogger]()),
			objectBean(&ZeroLogger{}).Name("logger2").Export(gs.As[CtxLogger]()),
			objectBean(new(struct {
				Loggers map[string]CtxLogger `inject:"*"`
			})),
		}
		err := r.Refresh(extractBeans(beans))
		assert.That(t, err).Nil()
	})

	t.Run("map injection non string key returns error", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		beans := []*gs_bean.BeanDefinition{
			objectBean(&ZeroLogger{}).Name("1").Export(gs.As[CtxLogger]()),
			objectBean(new(struct {
				Loggers map[int]CtxLogger `inject:"*"`
			})),
		}
		err := r.Refresh(extractBeans(beans))
		assert.Error(t, err).Matches("map key should be string")
	})

	t.Run("slice injection keeps explicit order", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		z := &ZeroLogger{}
		a := &SimpleLogger{}
		s := new(struct {
			Loggers []Logger `inject:"z,a"`
		})
		beans := []*gs_bean.BeanDefinition{
			objectBean(a).Name("a").Export(gs.As[Logger]()),
			objectBean(z).Name("z").Export(gs.As[Logger]()),
			objectBean(s),
		}
		err := r.Refresh(extractBeans(beans))
		assert.That(t, err).Nil()
		assert.That(t, len(s.Loggers)).Equal(2)
		assert.That(t, s.Loggers[0]).Equal(z)
		assert.That(t, s.Loggers[1]).Equal(a)
	})
}

func TestWireTag(t *testing.T) {

	t.Run("empty", func(t *testing.T) {
		tag := parseWireTag("")
		assert.That(t, tag).Equal(WireTag{})
		assert.That(t, tag.String()).Equal("")
	})

	t.Run("only name", func(t *testing.T) {
		tag := parseWireTag("a")
		assert.That(t, tag).Equal(WireTag{beanName: "a"})
		assert.That(t, tag.String()).Equal("a")
	})

	t.Run("only nullable", func(t *testing.T) {
		tag := parseWireTag("?")
		assert.That(t, tag).Equal(WireTag{nullable: true})
		assert.That(t, tag.String()).Equal("?")
	})

	t.Run("name and nullable", func(t *testing.T) {
		tag := parseWireTag("a?")
		assert.That(t, tag).Equal(WireTag{beanName: "a", nullable: true})
		assert.That(t, tag.String()).Equal("a?")
	})

	t.Run("tags - single", func(t *testing.T) {
		tags := []WireTag{
			{"a", true},
		}
		assert.That(t, toWireString(tags)).Equal("a?")
	})

	t.Run("tags - multiple", func(t *testing.T) {
		tags := []WireTag{
			{"a", true},
			{"b", false},
		}
		assert.That(t, toWireString(tags)).Equal("a?,b")
	})

	t.Run("tags - mixed nullable", func(t *testing.T) {
		tags := []WireTag{
			{"a", true},
			{"b", false},
			{"c", true},
		}
		assert.That(t, toWireString(tags)).Equal("a?,b,c?")
	})
}

type A struct {
	B *B `autowire:""`
}

type B struct {
	C *C `autowire:""`
}

type C struct {
	A *A `autowire:"?"`
	D *D `autowire:"?"`
}

type D struct {
	E *E `autowire:""`
}

type E struct {
	c *C
	g *G
}

func NewE(c *C, g *G) *E {
	return &E{c: c, g: g}
}

type F struct {
	G *G `autowire:""`
}

type G struct {
	e *E
}

func NewG(e *E) *G {
	return &G{e: e}
}

type H struct {
	i *I `autowire:""`
}

func NewH(i *I) *H {
	return &H{i: i}
}

type I struct {
	J *J `autowire:""`
}

type J struct {
	H *H `autowire:",lazy"`
}

func NewJ() *J {
	return &J{}
}

func TestCircularBean(t *testing.T) {

	t.Run("not truly circular - object", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		s := new(struct {
			A *A `autowire:""`
			B *B `autowire:""`
			C *C `autowire:""`
		})
		beans := []*gs_bean.BeanDefinition{
			objectBean(s),
			objectBean(&A{}),
			objectBean(&B{}),
			objectBean(&C{}),
		}
		err := r.Refresh(extractBeans(beans))
		assert.That(t, err).Nil()
		assert.That(t, s.A.B).Equal(s.B)
		assert.That(t, s.B.C).Equal(s.C)
		assert.That(t, s.C.A).Equal(s.A)
	})

	t.Run("not truly circular - constructor", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		s := new(struct {
			C *C `autowire:""`
			D *D `autowire:""`
			E *E `autowire:""`
		})
		beans := []*gs_bean.BeanDefinition{
			objectBean(s),
			objectBean(&C{}),
			objectBean(&D{}),
			provideBean(NewE, gs_arg.Index(1, gs_arg.Tag("?"))),
		}
		err := r.Refresh(extractBeans(beans))
		assert.That(t, err).Nil()
		assert.That(t, s.C.D).Equal(s.D)
		assert.That(t, s.D.E).Equal(s.E)
		assert.That(t, s.E.c).Equal(s.C)
	})

	t.Run("found circular - direct", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		beans := []*gs_bean.BeanDefinition{
			provideBean(NewE, gs_arg.Tag("?")),
			objectBean(&F{}),
			provideBean(NewG),
		}
		err := r.Refresh(extractBeans(beans))
		assert.Error(t, err).Matches("circular autowire dependency detected")
	})

	t.Run("found circular - indirect", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		beans := []*gs_bean.BeanDefinition{
			provideBean(NewH),
			objectBean(&I{}),
			provideBean(NewJ),
		}
		err := r.Refresh(extractBeans(beans))
		assert.Error(t, err).Nil()
	})

	t.Run("found circular - lazy", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{})))
		s := new(struct {
			H *H `autowire:""`
			I *I `autowire:""`
			J *J `autowire:""`
		})
		beans := []*gs_bean.BeanDefinition{
			objectBean(s),
			provideBean(NewH),
			objectBean(&I{}),
			provideBean(NewJ),
		}
		err := r.Refresh(extractBeans(beans))
		assert.That(t, err).Nil()
		assert.That(t, s.H.i).Equal(s.I)
		assert.That(t, s.I.J).Equal(s.J)
		assert.That(t, s.J.H).Equal(s.H)
	})
}

type Counter struct {
	count int
}

func (c *Counter) Incr() int {
	c.count++
	return c.count
}

type DestroyA struct {
	Counter *Counter `autowire:""`
	value   int
}

type DestroyB struct {
	Counter *Counter `autowire:""`
	value   int
}

func (d *DestroyB) Destroy() {
	d.value = d.Counter.Incr()
}

type DestroyC struct {
	Counter  *Counter  `autowire:""`
	DestroyD *DestroyD `autowire:""`
	value    int
}

type DestroyD struct {
	Counter  *Counter  `autowire:""`
	DestroyE *DestroyE `autowire:""`
}

type DestroyE struct {
	Counter *Counter `autowire:""`
	value   int
}

func (d *DestroyE) Destroy() {
	d.value = d.Counter.Incr()
}

func TestDestroy(t *testing.T) {

	t.Run("independent", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		s := new(struct {
			DestroyA *DestroyA `autowire:""`
			DestroyB *DestroyB `autowire:""`
		})
		beans := []*gs_bean.BeanDefinition{
			objectBean(s),
			objectBean(&Counter{}),
			objectBean(&DestroyA{}).Destroy(func(d *DestroyA) {
				d.value = d.Counter.Incr()
			}),
			objectBean(&DestroyB{}).DestroyMethod("Destroy"),
		}
		err := r.Refresh(extractBeans(beans))
		assert.That(t, err).Nil()
		r.Close()
		assert.That(t, s.DestroyA.value == 1 || s.DestroyA.value == 2).True()
		assert.That(t, s.DestroyB.value == 1 || s.DestroyB.value == 2).True()
	})

	t.Run("dependency", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
		s := new(struct {
			DestroyC *DestroyC `autowire:""`
			DestroyE *DestroyE `autowire:""`
		})
		beans := []*gs_bean.BeanDefinition{
			objectBean(s),
			objectBean(&Counter{}),
			objectBean(&DestroyC{}).Destroy(func(d *DestroyC) {
				d.value = d.Counter.Incr()
			}),
			objectBean(&DestroyD{}),
			objectBean(&DestroyE{}).DestroyMethod("Destroy"),
		}
		err := r.Refresh(extractBeans(beans))
		assert.That(t, err).Nil()
		r.Close()
		assert.That(t, s.DestroyC.value).Equal(1)
		assert.That(t, s.DestroyE.value).Equal(2)
	})
}

type DyncValue struct {
	Value gs_dync.Value[int] `value:"${:=3}"`
}

func TestDyncValue(t *testing.T) {

	t.Run("without dync value", func(t *testing.T) {
		release := make(chan string, 2)

		r := New(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{})))

		b1 := objectBean(&SimpleLogger{}).Name("biz")
		runtime.AddCleanup(&b1, func(s string) {
			release <- s
		}, "biz")

		b2 := objectBean(&SimpleLogger{}).Name("sys")
		runtime.AddCleanup(&b2, func(s string) {
			release <- s
		}, "sys")

		beans := []*gs_bean.BeanDefinition{b1, b2}
		err := r.Refresh(extractBeans(beans))
		assert.That(t, err).Nil()
		assert.That(t, r.p).Nil()
		//assert.That(t, r.beansByName).Nil()
		//assert.That(t, r.beansByType).Nil()

		runtime.GC()
		released := make(map[string]struct{})
		timeout := time.After(time.Second)
		for len(released) < 2 {
			select {
			case s := <-release:
				released[s] = struct{}{}
			case <-timeout:
				t.Fatal("timeout waiting for cleanup")
			}
		}
		assert.That(t, released).Equal(map[string]struct{}{
			"biz": {},
			"sys": {},
		})
	})

	t.Run("with dync value", func(t *testing.T) {
		r := New(flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{})))
		beans := []*gs_bean.BeanDefinition{
			objectBean(&DyncValue{}).Name("biz"),
			objectBean(&SimpleLogger{}).Name("sys"),
		}
		err := r.Refresh(extractBeans(beans))
		assert.That(t, err).Nil()
		assert.That(t, r.p).NotNil()
		//assert.That(t, r.beansByName).Nil()
		//assert.That(t, r.beansByType).Nil()
	})
}

func TestInjectionErrorFormat(t *testing.T) {
	// Config -> Service -> Repo (missing)
	type Repo struct{}
	type Service struct {
		Repo *Repo `autowire:""`
	}
	type Config struct {
		Service *Service `autowire:""`
	}

	// Register Config and Service, but missing Repo
	cfg := objectBean(&Config{}).Name("config")
	svc := objectBean(&Service{}).Name("service")

	beans := []*gs_bean.BeanDefinition{cfg, svc}
	roots, _ := extractBeans(beans)
	r := New(flatten.NewPropertiesStorage(flatten.NewProperties(nil)))
	err := r.Refresh(roots, beans)
	assert.That(t, err).NotNil()
}
