package config

import (
	"fmt"
	"io/ioutil"

	"githun.com/Shanjm/tracing-aspect/log"
	"golang.org/x/tools/go/ssa"
	yaml "gopkg.in/yaml.v3"
)

type ServerType int

const (
	O_HTTP ServerType = 0 << iota
	TRPC
)

type Config struct {
	App       string       `yaml:"name"`       // 应用名称
	Server    ServerType   `yaml:"type"`       // 应用网络类型
	NotCopy   bool         `yaml:"only_trace"` // 仅录制切面
	FList     []FuncConfig `yaml:"func_list"`  // 需要监听的函数
	WhiteList []string     `yaml:"white_list"` // 监听请求白名单
	BlackList []string     `yaml:"black_list"` // 监听请求黑名单
}

type FuncConfig struct {
	Name string `yaml:"name"`
}

type allConfig struct {
	ProjectConfig map[string]*Config `yaml:"project"`
}

func LoadLocal(app string) *Config {
	b, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		log.Panicln(err)
	}
	var configs *allConfig
	yaml.Unmarshal(b, &configs)
	if conf, ok := configs.ProjectConfig[app]; ok {
		return conf
	}
	log.Fatalln(fmt.Errorf("missing %s's config", app))
	return nil
}

func LoadOnline() *Config {
	// todo
	return nil
}

type Judger interface {
	IsHandler(fun *ssa.Function) bool
}

func (c *Config) GetHandlerJuder() Judger {
	switch c.Server {
	case O_HTTP:
		return new(originHttp)
	}

	return new(judger)
}

// 实现
type judger struct {
}

func (j *judger) IsHandler(fun *ssa.Function) bool {
	return false
}
