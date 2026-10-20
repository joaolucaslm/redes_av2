package testbed

import (
	"errors"
	"fmt"
	"io/fs"

	"bancada/internal/channel"
	"bancada/internal/fsm"
	"bancada/internal/host"
	"bancada/internal/rdt"
	"bancada/internal/tcp"
	"bancada/internal/udp"
)

var (
	ErrUnknownFamily = errors.New("família de protocolo desconhecida")
	ErrNotRunnable   = errors.New("arquivo é um componente e não pode rodar sozinho")
	ErrUnknownHost   = errors.New("host inexistente")
	ErrUnknownKind   = errors.New("tipo de comando desconhecido")
)

type Config struct {
	Protocol string         `json:"protocol"`
	Seed     uint64         `json:"seed"`
	Channel  channel.Params `json:"channel"`
	TCP      tcp.Config     `json:"tcp"`
	UDP      udp.Config     `json:"udp"`
	RDT      rdt.Config     `json:"rdt"`
}

func DefaultConfig(protocol string) Config {
	return Config{
		Protocol: protocol,
		Seed:     1,
		Channel:  channel.DefaultParams(),
		TCP:      tcp.DefaultConfig(),
		UDP:      udp.DefaultConfig(),
		RDT:      rdt.DefaultConfig(),
	}
}

type blueprint interface {
	build(env host.Env, cfg Config) (host.Stack, error)
}

type tcpBlueprint struct{ programs tcp.Programs }

func (b tcpBlueprint) build(env host.Env, cfg Config) (host.Stack, error) {
	return tcp.New(env, cfg.TCP, b.programs)
}

type udpBlueprint struct{ program *fsm.Program[*udp.Socket] }

func (b udpBlueprint) build(env host.Env, cfg Config) (host.Stack, error) {
	return udp.New(env, cfg.UDP, b.program)
}

type rdtBlueprint struct{ programs rdt.Programs }

func (b rdtBlueprint) build(env host.Env, cfg Config) (host.Stack, error) {
	return rdt.New(env, cfg.RDT, b.programs)
}

var families = map[string]func(cat *fsm.Catalog, protocol string) (blueprint, error){
	"tcp": func(cat *fsm.Catalog, protocol string) (blueprint, error) {
		programs, err := tcp.Compile(cat, protocol)
		return tcpBlueprint{programs}, err
	},
	"udp": func(cat *fsm.Catalog, protocol string) (blueprint, error) {
		program, err := udp.Compile(cat, protocol)
		return udpBlueprint{program}, err
	},
	"rdt": func(cat *fsm.Catalog, protocol string) (blueprint, error) {
		programs, err := rdt.Compile(cat, protocol)
		return rdtBlueprint{programs}, err
	},
}

func compile(cat *fsm.Catalog, protocol string) (fsm.File, blueprint, error) {
	file, err := cat.File(protocol)
	if err != nil {
		return fsm.File{}, nil, err
	}
	if file.Component {
		return file, nil, fmt.Errorf("%w: %s", ErrNotRunnable, protocol)
	}
	build, ok := families[file.Family]
	if !ok {
		return file, nil, fmt.Errorf("%w: %s (%s)", ErrUnknownFamily, file.Family, protocol)
	}
	bp, err := build(cat, protocol)
	if err != nil {
		return file, nil, err
	}
	return file, bp, nil
}

func LoadCatalog(fsys fs.FS) (*fsm.Catalog, error) {
	cat, err := fsm.LoadCatalog(fsys)
	if err != nil {
		return nil, err
	}
	var problems []error
	for _, file := range Runnable(cat) {
		if _, _, err := compile(cat, file.Protocol); err != nil {
			problems = append(problems, err)
		}
	}
	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}
	return cat, nil
}

func Runnable(cat *fsm.Catalog) []fsm.File {
	var runnable []fsm.File
	for _, file := range cat.Files() {
		if !file.Component {
			runnable = append(runnable, file)
		}
	}
	return runnable
}

func Family(cat *fsm.Catalog, protocol string) (string, error) {
	file, err := cat.File(protocol)
	if err != nil {
		return "", err
	}
	return file.Family, nil
}
