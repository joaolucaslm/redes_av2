package fsm

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrInvalidDefinition     = errors.New("definição de FSM inválida")
	ErrMissingField          = errors.New("campo obrigatório ausente")
	ErrUnknownState          = errors.New("estado inexistente")
	ErrDuplicateState        = errors.New("estado duplicado")
	ErrDuplicateMachine      = errors.New("máquina duplicada")
	ErrUnknownGuard          = errors.New("guarda não registrada")
	ErrUnknownAction         = errors.New("ação não registrada")
	ErrBadActionArgument     = errors.New("argumento de ação não permitido")
	ErrDuplicateTransition   = errors.New("transição duplicada")
	ErrUnreachableTransition = errors.New("transição inalcançável")
	ErrMalformedGuard        = errors.New("expressão de guarda malformada")
	ErrMalformedAction       = errors.New("referência de ação malformada")
	ErrUnknownMachine        = errors.New("máquina inexistente no arquivo")
	ErrUnknownProtocol       = errors.New("protocolo inexistente no catálogo")
	ErrDuplicateProtocol     = errors.New("protocolo declarado em mais de um arquivo")

	ErrUnknownEvent = errors.New("evento fora do alfabeto da máquina")
	ErrNotAccepted  = errors.New("nenhuma transição para este evento no estado atual")
	ErrGuardsFailed = errors.New("nenhuma guarda satisfeita")
	ErrReentrant    = errors.New("disparo reentrante na mesma máquina")
	ErrActionFailed = errors.New("ação falhou durante a transição")
)

type DefinitionError struct {
	Protocol string
	Machine  string
	Where    string
	Detail   string
	Err      error
}

func (e *DefinitionError) Error() string {
	var b strings.Builder
	b.WriteString(e.Protocol)
	if e.Machine != "" {
		b.WriteString("/" + e.Machine)
	}
	if e.Where != "" {
		b.WriteString(" [" + e.Where + "]")
	}
	b.WriteString(": " + e.Err.Error())
	if e.Detail != "" {
		b.WriteString(": " + e.Detail)
	}
	return b.String()
}

func (e *DefinitionError) Unwrap() []error {
	return []error{e.Err, ErrInvalidDefinition}
}

type RejectionError struct {
	Machine string
	State   State
	Event   Event
	Guards  []string
	Err     error
}

func (e *RejectionError) Error() string {
	return fmt.Sprintf("%s: evento %s rejeitado em %s: %s", e.Machine, e.Event, e.State, e.Reason())
}

func (e *RejectionError) Reason() string {
	if errors.Is(e.Err, ErrGuardsFailed) && len(e.Guards) > 0 {
		return fmt.Sprintf("%v (falsas: %s)", e.Err, strings.Join(e.Guards, " | "))
	}
	return e.Err.Error()
}

func (e *RejectionError) Unwrap() error { return e.Err }

type ActionError struct {
	Machine string
	Action  string
	Err     error
}

func (e *ActionError) Error() string {
	return fmt.Sprintf("%s: ação %s: %v", e.Machine, e.Action, e.Err)
}

func (e *ActionError) Unwrap() []error {
	return []error{e.Err, ErrActionFailed}
}
