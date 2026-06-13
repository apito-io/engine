package schema

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

const (
	StepBaseProjectDDL = "base_project_ddl"
	StepTenantDDL      = "tenant_ddl"
	StepSystemSchema   = "system_schema"
	StepCache          = "cache"
)

// Hooks wires tenant propagation and optional ledger persistence.
type Hooks struct {
	SchemaIterate func(ctx context.Context, project *models.Project, fn func(ctx context.Context, driver interface{}) error) error
	Store         interfaces.SchemaOperationsStore
	// AfterCommit runs after all orchestration steps succeed (base DDL, tenants, system persist, cache).
	AfterCommit func(in RunInput)
}

// RunInput is one synchronous schema mutation coordinated by the orchestrator.
type RunInput struct {
	Ctx           context.Context
	Project       *models.Project
	OperationType string
	Request       interface{}
	BaseDriver    interfaces.ProjectDBInterface
	SkipBaseDDL   bool
	ApplyDDL      func(driver interfaces.ProjectDBInterface) error
	PersistSystem func() error
	RefreshCache  func() error
	// Compensate runs on base project driver after partial success (best-effort).
	Compensate func(driver interfaces.ProjectDBInterface) error
	// PhysicalDDLRequired when true tells PostSchemaChangeHook to flush project/tenant replicas.
	PhysicalDDLRequired bool
}

// Run executes base DDL → tenant DDL → system persist → cache with optional ledger + compensation.
func Run(h Hooks, in RunInput) error {
	if in.Ctx == nil {
		return fmt.Errorf("schema orchestrator: context required")
	}
	if in.Project == nil {
		return fmt.Errorf("schema orchestrator: project required")
	}
	if !in.SkipBaseDDL && in.BaseDriver == nil {
		return fmt.Errorf("schema orchestrator: base project driver required")
	}
	if in.ApplyDDL == nil {
		return fmt.Errorf("schema orchestrator: ApplyDDL required")
	}
	if in.PersistSystem == nil {
		return fmt.Errorf("schema orchestrator: PersistSystem required")
	}
	if in.RefreshCache == nil {
		return fmt.Errorf("schema orchestrator: RefreshCache required")
	}

	var op *models.SchemaOperation
	if h.Store != nil {
		var err error
		op, err = beginOperation(h.Store, in)
		if err != nil {
			return err
		}
	}

	steps := []models.SchemaOperationStep{
		{Key: StepBaseProjectDDL, State: models.SchemaOpStepPending},
		{Key: StepTenantDDL, State: models.SchemaOpStepPending},
		{Key: StepSystemSchema, State: models.SchemaOpStepPending},
		{Key: StepCache, State: models.SchemaOpStepPending},
	}

	mark := func(key, state, errMsg string) {
		for i := range steps {
			if steps[i].Key == key {
				steps[i].State = state
				steps[i].Error = errMsg
				steps[i].Updated = utility.GetCurrentTime()
				break
			}
		}
		if op != nil {
			_ = op.SetSteps(steps)
			op.UpdatedAt = utility.GetCurrentTime()
			_ = h.Store.UpdateSchemaOperation(in.Ctx, op)
		}
	}

	if !in.SkipBaseDDL {
		if err := in.ApplyDDL(in.BaseDriver); err != nil {
			mark(StepBaseProjectDDL, models.SchemaOpStepFailed, err.Error())
			finishOperation(h.Store, in.Ctx, op, models.SchemaOpStatusFailed, err)
			return fmt.Errorf("base project schema DDL: %w", err)
		}
	}
	mark(StepBaseProjectDDL, models.SchemaOpStepSucceeded, "")

	if err := applyTenantDDL(h, in); err != nil {
		mark(StepTenantDDL, models.SchemaOpStepFailed, err.Error())
		runCompensation(in, h, op, steps, mark)
		finishOperation(h.Store, in.Ctx, op, models.SchemaOpStatusFailed, err)
		return fmt.Errorf("tenant schema propagation: %w", err)
	}
	mark(StepTenantDDL, models.SchemaOpStepSucceeded, "")

	if err := in.PersistSystem(); err != nil {
		mark(StepSystemSchema, models.SchemaOpStepFailed, err.Error())
		runCompensation(in, h, op, steps, mark)
		finishOperation(h.Store, in.Ctx, op, statusAfterCompensation(in.Compensate), err)
		return fmt.Errorf("persist system schema: %w", err)
	}
	mark(StepSystemSchema, models.SchemaOpStepSucceeded, "")

	if err := in.RefreshCache(); err != nil {
		mark(StepCache, models.SchemaOpStepFailed, err.Error())
		finishOperation(h.Store, in.Ctx, op, models.SchemaOpStatusNeedsRepair, err)
		return fmt.Errorf("refresh schema cache: %w", err)
	}
	mark(StepCache, models.SchemaOpStepSucceeded, "")
	finishOperation(h.Store, in.Ctx, op, models.SchemaOpStatusCommitted, nil)
	if h.AfterCommit != nil {
		h.AfterCommit(in)
	}
	return nil
}

func applyTenantDDL(h Hooks, in RunInput) error {
	if h.SchemaIterate == nil {
		return nil
	}
	return h.SchemaIterate(in.Ctx, in.Project, func(ctx context.Context, drv interface{}) error {
		td, ok := drv.(interfaces.ProjectDBInterface)
		if !ok || td == nil {
			return nil
		}
		return in.ApplyDDL(td)
	})
}

func runCompensation(in RunInput, h Hooks, op *models.SchemaOperation, steps []models.SchemaOperationStep, mark func(string, string, string)) {
	if in.SkipBaseDDL || in.Compensate == nil {
		if op != nil {
			op.Status = models.SchemaOpStatusNeedsRepair
		}
		return
	}
	if op != nil {
		op.Status = models.SchemaOpStatusCompensating
		op.UpdatedAt = utility.GetCurrentTime()
		_ = h.Store.UpdateSchemaOperation(in.Ctx, op)
	}
	if err := in.Compensate(in.BaseDriver); err != nil {
		mark(StepBaseProjectDDL, models.SchemaOpStepFailed, "compensation: "+err.Error())
		if op != nil {
			op.Status = models.SchemaOpStatusNeedsRepair
			op.Error = err.Error()
		}
		return
	}
	mark(StepBaseProjectDDL, models.SchemaOpStepCompensated, "")
	_ = applyTenantDDL(h, RunInput{
		Ctx: in.Ctx, Project: in.Project, BaseDriver: in.BaseDriver,
		ApplyDDL: in.Compensate,
	})
}

func statusAfterCompensation(compensate func(interfaces.ProjectDBInterface) error) string {
	if compensate == nil {
		return models.SchemaOpStatusNeedsRepair
	}
	return models.SchemaOpStatusFailed
}

func beginOperation(store interfaces.SchemaOperationsStore, in RunInput) (*models.SchemaOperation, error) {
	reqJSON, _ := json.Marshal(in.Request)
	beforeJSON, _ := json.Marshal(in.Project.Schema)
	now := utility.GetCurrentTime()
	op := &models.SchemaOperation{
		ID:               utility.NewID(),
		ProjectID:        in.Project.ID,
		OperationType:    in.OperationType,
		Status:           models.SchemaOpStatusApplying,
		RequestJSON:      string(reqJSON),
		BeforeSchemaJSON: string(beforeJSON),
		AttemptCount:     1,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := op.SetSteps([]models.SchemaOperationStep{
		{Key: StepBaseProjectDDL, State: models.SchemaOpStepPending},
		{Key: StepTenantDDL, State: models.SchemaOpStepPending},
		{Key: StepSystemSchema, State: models.SchemaOpStepPending},
		{Key: StepCache, State: models.SchemaOpStepPending},
	}); err != nil {
		return nil, err
	}
	if err := store.CreateSchemaOperation(in.Ctx, op); err != nil {
		return nil, err
	}
	return op, nil
}

func finishOperation(store interfaces.SchemaOperationsStore, ctx context.Context, op *models.SchemaOperation, status string, err error) {
	if store == nil || op == nil {
		return
	}
	op.Status = status
	op.UpdatedAt = utility.GetCurrentTime()
	if err != nil {
		op.Error = err.Error()
	} else {
		op.Error = ""
	}
	_ = store.UpdateSchemaOperation(ctx, op)
}

// StoreFromSystemDriver returns SchemaOperationsStore when the system driver implements it.
func StoreFromSystemDriver(sys interfaces.ApitoSystemDB) interfaces.SchemaOperationsStore {
	if sys == nil {
		return nil
	}
	if s, ok := sys.(interfaces.SchemaOperationsStore); ok {
		return s
	}
	return nil
}
