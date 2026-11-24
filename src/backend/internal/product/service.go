package product

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hieumai1507/hpay-backend/internal/listing"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
)

type Service struct {
	pool *pgxpool.Pool
	q    *store.Queries
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{
		pool: pool,
		q:    store.New(pool),
	}
}

// ListOrganizations is the admin-ops equivalent of org.Service.
// ListUserOrganizations — every org, not just ones the caller belongs to, so
// it works for an admin-audience caller who has no org_members row at all.
func (s *Service) ListOrganizations(ctx context.Context, f listing.Filter) ([]store.ListOrganizationsAdminRow, int64, error) {
	f = f.Normalize()
	rows, err := s.q.ListOrganizationsAdmin(ctx, store.ListOrganizationsAdminParams{
		Name:      f.Text(),
		SortOrder: f.SortOrder(),
		Lim:       f.Limit,
		Off:       f.Offset,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountOrganizationsAdmin(ctx, f.Text())
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// ListServices returns one page plus the total row count for the same filter,
// so the caller can report honest pagination metadata.
func (s *Service) ListServices(ctx context.Context, f listing.Filter) ([]store.Service, int64, error) {
	f = f.Normalize()
	rows, err := s.q.ListServices(ctx, store.ListServicesParams{
		Name:      f.Text(),
		SortOrder: f.SortOrder(),
		Lim:       f.Limit,
		Off:       f.Offset,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountServices(ctx, f.Text())
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (s *Service) GetService(ctx context.Context, id uuid.UUID) (store.Service, error) {
	return s.q.GetService(ctx, id)
}

func (s *Service) CreateService(ctx context.Context, code, name, category string, priority string, channels []byte) (store.Service, error) {
	if priority == "" {
		priority = "before_gl"
	}
	if len(channels) == 0 {
		channels = []byte(`["Mobile"]`)
	}
	return s.q.InsertService(ctx, store.InsertServiceParams{
		Code:           code,
		Name:           name,
		Category:       category,
		Status:         store.ServiceStatusActive,
		ActionPriority: priority,
		Channels:       channels,
	})
}

func (s *Service) UpdateServiceStatus(ctx context.Context, id uuid.UUID, status store.ServiceStatus) (store.Service, error) {
	return s.q.UpdateServiceStatus(ctx, store.UpdateServiceStatusParams{
		ID:     id,
		Status: status,
	})
}

func (s *Service) ListOffers(ctx context.Context, f listing.Filter) ([]store.ListOffersRow, int64, error) {
	f = f.Normalize()
	rows, err := s.q.ListOffers(ctx, store.ListOffersParams{
		Name: f.Text(),
		Lim:  f.Limit,
		Off:  f.Offset,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountOffers(ctx, f.Text())
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (s *Service) GetOfferDetail(ctx context.Context, offerID uuid.UUID) (store.Offer, []store.ListOfferServicesRow, error) {
	off, err := s.q.GetOffer(ctx, offerID)
	if err != nil {
		return store.Offer{}, nil, err
	}
	srvs, err := s.q.ListOfferServices(ctx, offerID)
	if err != nil {
		return store.Offer{}, nil, err
	}
	return off, srvs, nil
}

func (s *Service) CreateOffer(ctx context.Context, code, name string, role store.OfferRole, level int32, active bool, maxPocket int64, maxBanks int32, updatedBy string) (store.Offer, error) {
	return s.q.InsertOffer(ctx, store.InsertOfferParams{
		Code:             code,
		Name:             name,
		Role:             role,
		Level:            level,
		IsActive:         active,
		PrimaryPocketMax: maxPocket,
		MaxBankAccounts:  maxBanks,
		UpdatedBy:        updatedBy,
	})
}

func (s *Service) UpdateOffer(ctx context.Context, id uuid.UUID, name string, role store.OfferRole, level int32, active bool, maxPocket int64, maxBanks int32, updatedBy string) (store.Offer, error) {
	return s.q.UpdateOffer(ctx, store.UpdateOfferParams{
		ID:               id,
		Name:             name,
		Role:             role,
		Level:            level,
		IsActive:         active,
		PrimaryPocketMax: maxPocket,
		MaxBankAccounts:  maxBanks,
		UpdatedBy:        updatedBy,
	})
}

func (s *Service) ListAuditLogs(ctx context.Context, f listing.Filter) ([]store.AuditLog, int64, error) {
	f = f.Normalize()
	logs, err := s.q.ListAuditLogs(ctx, store.ListAuditLogsParams{
		Name:      f.Text(),
		FromAt:    f.FromTS(),
		ToAt:      f.ToTS(),
		SortOrder: f.SortOrder(),
		Lim:       f.Limit,
		Off:       f.Offset,
	})
	if err != nil {
		return nil, 0, err
	}
	count, err := s.q.CountAuditLogs(ctx, store.CountAuditLogsParams{
		Name: f.Text(), FromAt: f.FromTS(), ToAt: f.ToTS(),
	})
	if err != nil {
		return nil, 0, err
	}
	return logs, count, nil
}

// WorkflowInstanceFilter carries the 3 controls already on WorkflowPage.tsx
// (docs plan workflow-instances-filters) that sit outside the shared
// listing.Filter shape.
type WorkflowInstanceFilter struct {
	Status     string // "" = any; short UI label, mapped to the wf_status enum by the caller
	Definition string // definition name needle, e.g. "p2p_transfer"
	StuckOnly  bool   // status=running AND older than 5 minutes
}

func (s *Service) ListWorkflowInstances(ctx context.Context, f listing.Filter, wf WorkflowInstanceFilter) ([]store.ListWorkflowInstancesRow, int64, error) {
	f = f.Normalize()
	status := pgtype.Text{}
	if wf.Status != "" {
		status = pgtype.Text{String: wf.Status, Valid: true}
	}
	definition := pgtype.Text{}
	if wf.Definition != "" {
		definition = pgtype.Text{String: wf.Definition, Valid: true}
	}
	rows, err := s.q.ListWorkflowInstances(ctx, store.ListWorkflowInstancesParams{
		Status: status, Definition: definition, StuckOnly: wf.StuckOnly,
		Name: f.Text(), FromAt: f.FromTS(), ToAt: f.ToTS(),
		SortOrder: f.SortOrder(), Lim: f.Limit, Off: f.Offset,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountWorkflowInstances(ctx, store.CountWorkflowInstancesParams{
		Status: status, Definition: definition, StuckOnly: wf.StuckOnly,
		Name: f.Text(), FromAt: f.FromTS(), ToAt: f.ToTS(),
	})
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (s *Service) GetWorkflowInstanceDetail(ctx context.Context, id uuid.UUID) (store.GetInstanceRow, []store.ListStepExecutionsRow, error) {
	inst, err := s.q.GetInstance(ctx, id)
	if err != nil {
		return store.GetInstanceRow{}, nil, err
	}
	steps, err := s.q.ListStepExecutions(ctx, id)
	if err != nil {
		return store.GetInstanceRow{}, nil, err
	}
	return inst, steps, nil
}

func (s *Service) ListFraudFlags(ctx context.Context, status string, f listing.Filter) ([]store.ListFraudFlagsRow, int64, error) {
	f = f.Normalize()
	if status == "" || status == "all" {
		rows, err := s.q.ListAllFraudFlags(ctx, store.ListAllFraudFlagsParams{
			RefCode:   f.Text(),
			FromAt:    f.FromTS(),
			ToAt:      f.ToTS(),
			SortOrder: f.SortOrder(),
			Lim:       f.Limit,
			Off:       f.Offset,
		})
		if err != nil {
			return nil, 0, err
		}
		res := make([]store.ListFraudFlagsRow, 0, len(rows))
		for _, r := range rows {
			res = append(res, store.ListFraudFlagsRow{
				ID:            r.ID,
				TransactionID: r.TransactionID,
				RefCode:       r.RefCode,
				RuleCode:      r.RuleCode,
				Score:         r.Score,
				Reasons:       r.Reasons,
				Status:        r.Status,
				CreatedAt:     r.CreatedAt,
			})
		}
		total, err := s.q.CountAllFraudFlags(ctx, store.CountAllFraudFlagsParams{
			RefCode: f.Text(), FromAt: f.FromTS(), ToAt: f.ToTS(),
		})
		if err != nil {
			return nil, 0, err
		}
		return res, total, nil
	}
	st := store.FlagStatusOpen
	if status == "cleared" {
		st = store.FlagStatusCleared
	} else if status == "confirmed" || status == "confirmed_fraud" {
		st = store.FlagStatusConfirmedFraud
	}
	rows, err := s.q.ListFraudFlags(ctx, store.ListFraudFlagsParams{
		Status:    st,
		RefCode:   f.Text(),
		FromAt:    f.FromTS(),
		ToAt:      f.ToTS(),
		SortOrder: f.SortOrder(),
		Lim:       f.Limit,
		Off:       f.Offset,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountFraudFlags(ctx, store.CountFraudFlagsParams{
		Status: st, RefCode: f.Text(), FromAt: f.FromTS(), ToAt: f.ToTS(),
	})
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (s *Service) RecordAudit(ctx context.Context, actorID uuid.UUID, actorType, action, resource, reason string) error {
	return s.q.InsertAudit(ctx, store.InsertAuditParams{
		ActorID:   actorID,
		ActorType: actorType,
		Action:    action,
		Resource:  resource,
		Reason:    pgtype.Text{String: reason, Valid: reason != ""},
	})
}

// AttentionSummary counts the parked money cases an operator must resolve by hand
// (G12): approvals parked as needs_manual and workflow instances in compensation_failed.
type AttentionSummary struct {
	NeedsManualApprovals        int64 `json:"needs_manual_approvals"`
	CompensationFailedWorkflows int64 `json:"compensation_failed_workflows"`
	Total                       int64 `json:"total"`
}

func (s *Service) AttentionSummary(ctx context.Context) (AttentionSummary, error) {
	r, err := s.q.CountAttentionItems(ctx)
	if err != nil {
		return AttentionSummary{}, err
	}
	return AttentionSummary{
		NeedsManualApprovals:        r.NeedsManualApprovals,
		CompensationFailedWorkflows: r.CompensationFailedWorkflows,
		Total:                       r.NeedsManualApprovals + r.CompensationFailedWorkflows,
	}, nil
}
