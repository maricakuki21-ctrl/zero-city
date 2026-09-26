package mediatask

import (
	"context"
	"errors"
	"strings"
)

var (
	ErrCreateInProgress = errors.New("media create is already in progress")
	ErrInvalidTask      = errors.New("media task is invalid")
)

type State string

const (
	StateCreating   State = "creating"
	StateAccepted   State = "accepted"
	StateProcessing State = "processing"
	StateSucceeded  State = "succeeded"
	StateFailed     State = "failed"
	StateCancelled  State = "cancelled"
)

func (s State) Terminal() bool {
	return s == StateSucceeded || s == StateFailed || s == StateCancelled
}

type Task struct {
	Context           CreateContext
	UpstreamTaskID    string
	State             State
	SettlementEventID string
}

type Observation struct {
	State State
}

type Store interface {
	ClaimCreate(context.Context, CreateContext) (Task, bool, error)
	Accept(context.Context, CreateContext, string) (Task, error)
	Get(context.Context, CreateContext) (Task, error)
	Finalize(context.Context, CreateContext, Observation) (Task, bool, error)
}

type Upstream interface {
	Create(context.Context, CreateContext) (string, error)
	Status(context.Context, string) (Observation, error)
	Cancel(context.Context, string) (Observation, error)
	Content(context.Context, string) ([]byte, error)
}

type SettlementSink interface {
	Apply(context.Context, Task) error
}

type Coordinator struct {
	store      Store
	upstream   Upstream
	settlement SettlementSink
}

func NewCoordinator(store Store, upstream Upstream, settlement SettlementSink) *Coordinator {
	return &Coordinator{store: store, upstream: upstream, settlement: settlement}
}

func (c *Coordinator) Create(ctx context.Context, createContext CreateContext) (Task, error) {
	task, claimed, err := c.store.ClaimCreate(ctx, createContext)
	if err != nil {
		return Task{}, err
	}
	if strings.TrimSpace(task.UpstreamTaskID) != "" {
		return task, nil
	}
	if !claimed {
		return Task{}, ErrCreateInProgress
	}
	upstreamTaskID, err := c.upstream.Create(ctx, createContext)
	if err != nil {
		return Task{}, err
	}
	if strings.TrimSpace(upstreamTaskID) == "" {
		return Task{}, ErrInvalidTask
	}
	return c.store.Accept(ctx, createContext, upstreamTaskID)
}

func (c *Coordinator) Status(ctx context.Context, createContext CreateContext) (Task, error) {
	return c.observe(ctx, createContext, c.upstream.Status)
}

func (c *Coordinator) Cancel(ctx context.Context, createContext CreateContext) (Task, error) {
	return c.observe(ctx, createContext, c.upstream.Cancel)
}

func (c *Coordinator) Content(ctx context.Context, createContext CreateContext) ([]byte, error) {
	task, err := c.store.Get(ctx, createContext)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(task.UpstreamTaskID) == "" {
		return nil, ErrInvalidTask
	}
	return c.upstream.Content(ctx, task.UpstreamTaskID)
}

func (c *Coordinator) observe(
	ctx context.Context,
	createContext CreateContext,
	operation func(context.Context, string) (Observation, error),
) (Task, error) {
	task, err := c.store.Get(ctx, createContext)
	if err != nil {
		return Task{}, err
	}
	if task.State.Terminal() {
		return task, nil
	}
	observation, err := operation(ctx, task.UpstreamTaskID)
	if err != nil {
		return Task{}, err
	}
	finalized, applied, err := c.store.Finalize(ctx, createContext, observation)
	if err != nil || !applied || !finalized.State.Terminal() {
		return finalized, err
	}
	if c.settlement != nil {
		if err := c.settlement.Apply(ctx, finalized); err != nil {
			return Task{}, err
		}
	}
	return finalized, nil
}
