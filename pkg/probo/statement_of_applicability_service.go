// Copyright (c) 2026 Probo Inc <hello@getprobo.com>.
//
// Permission to use, copy, modify, and/or distribute this software for any
// purpose with or without fee is hereby granted, provided that the above
// copyright notice and this permission notice appear in all copies.
//
// THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES WITH
// REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY
// AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY SPECIAL, DIRECT,
// INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM
// LOSS OF USE, DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR
// OTHER TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR
// PERFORMANCE OF THIS SOFTWARE.

package probo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"go.gearno.de/kit/pg"
	"go.probo.inc/probo/pkg/coredata"
	"go.probo.inc/probo/pkg/docgen"
	"go.probo.inc/probo/pkg/gid"
	"go.probo.inc/probo/pkg/html2pdf"
	"go.probo.inc/probo/pkg/page"
	"go.probo.inc/probo/pkg/prosemirror"
	"go.probo.inc/probo/pkg/validator"
)

type StatementOfApplicabilityService struct {
	svc               *TenantService
	html2pdfConverter *html2pdf.Converter
}

type (
	CreateStatementOfApplicabilityRequest struct {
		OrganizationID     gid.GID
		Name               string
		DefaultApproverIDs []gid.GID
	}

	UpdateStatementOfApplicabilityRequest struct {
		StatementOfApplicabilityID gid.GID
		Name                       *string
		DefaultApproverIDs         *[]gid.GID
	}
)

func (csr *CreateStatementOfApplicabilityRequest) Validate() error {
	v := validator.New()

	v.Check(csr.OrganizationID, "organization_id", validator.Required(), validator.GID(coredata.OrganizationEntityType))
	v.Check(csr.Name, "name", validator.SafeTextNoNewLine(TitleMaxLength))
	v.Check(len(csr.DefaultApproverIDs), "default_approver_ids", validator.Max(100))
	v.Check(csr.DefaultApproverIDs, "default_approver_ids", validator.NoDuplicates())
	v.CheckEach(csr.DefaultApproverIDs, "default_approver_ids", func(_ int, item any) {
		v.Check(item, "default_approver_ids", validator.GID(coredata.MembershipProfileEntityType))
	})

	return v.Error()
}

func (usr *UpdateStatementOfApplicabilityRequest) Validate() error {
	v := validator.New()

	v.Check(usr.StatementOfApplicabilityID, "statement_of_applicability_id", validator.Required(), validator.GID(coredata.StatementOfApplicabilityEntityType))
	v.Check(usr.Name, "name", validator.SafeTextNoNewLine(TitleMaxLength))
	if usr.DefaultApproverIDs != nil {
		v.Check(len(*usr.DefaultApproverIDs), "default_approver_ids", validator.Max(100))
		v.Check(*usr.DefaultApproverIDs, "default_approver_ids", validator.NoDuplicates())
		v.CheckEach(*usr.DefaultApproverIDs, "default_approver_ids", func(_ int, item any) {
			v.Check(item, "default_approver_ids", validator.GID(coredata.MembershipProfileEntityType))
		})
	}

	return v.Error()
}

func (s StatementOfApplicabilityService) ListForOrganizationID(
	ctx context.Context,
	organizationID gid.GID,
	cursor *page.Cursor[coredata.StatementOfApplicabilityOrderField],
	filter *coredata.StatementOfApplicabilityFilter,
) (*page.Page[*coredata.StatementOfApplicability, coredata.StatementOfApplicabilityOrderField], error) {
	var statementsOfApplicability coredata.StatementsOfApplicability
	organization := &coredata.Organization{}

	err := s.svc.pg.WithConn(
		ctx,
		func(ctx context.Context, conn pg.Querier) error {
			if err := organization.LoadByID(ctx, conn, s.svc.scope, organizationID); err != nil {
				return fmt.Errorf("cannot load organization: %w", err)
			}

			err := statementsOfApplicability.LoadByOrganizationID(
				ctx,
				conn,
				s.svc.scope,
				organization.ID,
				cursor,
				filter,
			)
			if err != nil {
				return fmt.Errorf("cannot load statements_of_applicability: %w", err)
			}

			return nil
		},
	)

	if err != nil {
		return nil, err
	}

	return page.NewPage(statementsOfApplicability, cursor), nil
}

func (s StatementOfApplicabilityService) CountForOrganizationID(
	ctx context.Context,
	organizationID gid.GID,
	filter *coredata.StatementOfApplicabilityFilter,
) (int, error) {
	var count int

	err := s.svc.pg.WithConn(
		ctx,
		func(ctx context.Context, conn pg.Querier) (err error) {
			statementsOfApplicability := &coredata.StatementsOfApplicability{}
			count, err = statementsOfApplicability.CountByOrganizationID(ctx, conn, s.svc.scope, organizationID, filter)
			if err != nil {
				return fmt.Errorf("cannot count statements_of_applicability: %w", err)
			}

			return nil
		},
	)

	if err != nil {
		return 0, err
	}

	return count, nil
}

func (s StatementOfApplicabilityService) Get(
	ctx context.Context,
	statementOfApplicabilityID gid.GID,
) (*coredata.StatementOfApplicability, error) {
	statementOfApplicability := &coredata.StatementOfApplicability{}

	err := s.svc.pg.WithConn(
		ctx,
		func(ctx context.Context, conn pg.Querier) error {
			return statementOfApplicability.LoadByID(ctx, conn, s.svc.scope, statementOfApplicabilityID)
		},
	)

	if err != nil {
		return nil, err
	}

	return statementOfApplicability, nil
}

func (s StatementOfApplicabilityService) GetDefaultApprovers(
	ctx context.Context,
	statementOfApplicabilityID gid.GID,
) (coredata.MembershipProfiles, error) {
	var approvers coredata.StatementOfApplicabilityDefaultApprovers

	err := s.svc.pg.WithConn(
		ctx,
		func(ctx context.Context, conn pg.Querier) error {
			return approvers.LoadByStatementOfApplicabilityID(ctx, conn, s.svc.scope, statementOfApplicabilityID)
		},
	)

	if err != nil {
		return nil, fmt.Errorf("cannot load default approvers: %w", err)
	}

	if len(approvers) == 0 {
		return nil, nil
	}

	profileIDs := make([]gid.GID, len(approvers))
	for i, a := range approvers {
		profileIDs[i] = a.ApproverProfileID
	}

	var profiles coredata.MembershipProfiles

	err = s.svc.pg.WithConn(
		ctx,
		func(ctx context.Context, conn pg.Querier) error {
			return profiles.LoadByIDs(ctx, conn, s.svc.scope, profileIDs)
		},
	)

	if err != nil {
		return nil, fmt.Errorf("cannot load approver profiles: %w", err)
	}

	return profiles, nil
}

func (s StatementOfApplicabilityService) Create(
	ctx context.Context,
	req CreateStatementOfApplicabilityRequest,
) (*coredata.StatementOfApplicability, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	now := time.Now()
	organization := &coredata.Organization{}

	err := s.svc.pg.WithConn(
		ctx,
		func(ctx context.Context, conn pg.Querier) error {
			return organization.LoadByID(ctx, conn, s.svc.scope, req.OrganizationID)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("cannot load organization: %w", err)
	}

	statementOfApplicabilityID := gid.New(organization.ID.TenantID(), coredata.StatementOfApplicabilityEntityType)
	statementOfApplicability := &coredata.StatementOfApplicability{
		ID:             statementOfApplicabilityID,
		OrganizationID: organization.ID,
		Name:           req.Name,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	err = s.svc.pg.WithTx(
		ctx,
		func(ctx context.Context, conn pg.Tx) error {
			if err := statementOfApplicability.Insert(ctx, conn, s.svc.scope); err != nil {
				return fmt.Errorf("cannot insert statement_of_applicability: %w", err)
			}

			if len(req.DefaultApproverIDs) > 0 {
				approvers := &coredata.StatementOfApplicabilityDefaultApprovers{}
				if err := approvers.MergeByStatementOfApplicabilityID(
				ctx,
				conn,
				s.svc.scope,
				statementOfApplicabilityID,
				organization.ID,
				req.DefaultApproverIDs,
			); err != nil {
					return fmt.Errorf("cannot set default approvers: %w", err)
				}
			}

			return nil
		},
	)

	if err != nil {
		return nil, err
	}

	return statementOfApplicability, nil
}

func (s StatementOfApplicabilityService) Update(
	ctx context.Context,
	req UpdateStatementOfApplicabilityRequest,
) (*coredata.StatementOfApplicability, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	statementOfApplicability := &coredata.StatementOfApplicability{}

	err := s.svc.pg.WithTx(
		ctx,
		func(ctx context.Context, conn pg.Tx) error {
			if err := statementOfApplicability.LoadByID(ctx, conn, s.svc.scope, req.StatementOfApplicabilityID); err != nil {
				return fmt.Errorf("cannot load statement_of_applicability: %w", err)
			}

			if req.Name != nil {
				statementOfApplicability.Name = *req.Name
			}

			statementOfApplicability.UpdatedAt = time.Now()

			if err := statementOfApplicability.Update(ctx, conn, s.svc.scope); err != nil {
				return fmt.Errorf("cannot update statement_of_applicability: %w", err)
			}

			if req.DefaultApproverIDs != nil {
				approvers := &coredata.StatementOfApplicabilityDefaultApprovers{}
				if err := approvers.MergeByStatementOfApplicabilityID(
				ctx,
				conn,
				s.svc.scope,
				req.StatementOfApplicabilityID,
				statementOfApplicability.OrganizationID,
				*req.DefaultApproverIDs,
			); err != nil {
					return fmt.Errorf("cannot update default approvers: %w", err)
				}
			}

			return nil
		},
	)

	if err != nil {
		return nil, err
	}

	return statementOfApplicability, nil
}

func (s StatementOfApplicabilityService) Delete(
	ctx context.Context,
	statementOfApplicabilityID gid.GID,
) error {
	statementOfApplicability := &coredata.StatementOfApplicability{ID: statementOfApplicabilityID}

	err := s.svc.pg.WithTx(
		ctx,
		func(ctx context.Context, conn pg.Tx) error {
			if err := statementOfApplicability.LoadByID(ctx, conn, s.svc.scope, statementOfApplicabilityID); err != nil {
				return fmt.Errorf("cannot load statement_of_applicability: %w", err)
			}

			if err := statementOfApplicability.Delete(ctx, conn, s.svc.scope); err != nil {
				return fmt.Errorf("cannot delete statement_of_applicability: %w", err)
			}

			return nil
		},
	)

	if err != nil {
		return err
	}

	return nil
}

func (s StatementOfApplicabilityService) GetApplicabilityStatement(
	ctx context.Context,
	applicabilityStatementID gid.GID,
) (*coredata.ApplicabilityStatement, error) {
	applicabilityStatement := &coredata.ApplicabilityStatement{}

	err := s.svc.pg.WithConn(
		ctx,
		func(ctx context.Context, conn pg.Querier) error {
			return applicabilityStatement.LoadByID(ctx, conn, s.svc.scope, applicabilityStatementID)
		},
	)
	if err != nil {
		return nil, err
	}

	return applicabilityStatement, nil
}

func (s StatementOfApplicabilityService) ListApplicabilityStatements(
	ctx context.Context,
	statementOfApplicabilityID gid.GID,
	cursor *page.Cursor[coredata.ApplicabilityStatementOrderField],
) (*page.Page[*coredata.ApplicabilityStatement, coredata.ApplicabilityStatementOrderField], error) {
	var statements coredata.ApplicabilityStatements

	err := s.svc.pg.WithConn(
		ctx,
		func(ctx context.Context, conn pg.Querier) error {
			if err := statements.LoadByStatementOfApplicabilityID(ctx, conn, s.svc.scope, statementOfApplicabilityID, cursor); err != nil {
				return fmt.Errorf("cannot load applicability statements: %w", err)
			}
			return nil
		},
	)

	if err != nil {
		return nil, err
	}

	return page.NewPage(statements, cursor), nil
}

func (s StatementOfApplicabilityService) CountApplicabilityStatements(
	ctx context.Context,
	statementOfApplicabilityID gid.GID,
) (int, error) {
	var count int

	err := s.svc.pg.WithConn(
		ctx,
		func(ctx context.Context, conn pg.Querier) (err error) {
			statements := &coredata.ApplicabilityStatements{}
			count, err = statements.CountByStatementOfApplicabilityID(ctx, conn, s.svc.scope, statementOfApplicabilityID)
			if err != nil {
				return fmt.Errorf("cannot count applicability statements: %w", err)
			}
			return nil
		},
	)

	if err != nil {
		return 0, err
	}

	return count, nil
}

func (s StatementOfApplicabilityService) CreateApplicabilityStatement(
	ctx context.Context,
	statementOfApplicabilityID gid.GID,
	controlID gid.GID,
	applicability bool,
	justification *string,
) (*coredata.ApplicabilityStatement, error) {
	var (
		statementOfApplicability = &coredata.StatementOfApplicability{}
		applicabilityStatement   = &coredata.ApplicabilityStatement{}
		now                      = time.Now()
	)

	err := s.svc.pg.WithTx(
		ctx,
		func(ctx context.Context, conn pg.Tx) error {
			if err := statementOfApplicability.LoadByID(ctx, conn, s.svc.scope, statementOfApplicabilityID); err != nil {
				return fmt.Errorf("cannot load statement of applicability: %w", err)
			}

			applicabilityStatement = &coredata.ApplicabilityStatement{
				ID:                         gid.New(s.svc.scope.GetTenantID(), coredata.ApplicabilityStatementEntityType),
				StatementOfApplicabilityID: statementOfApplicabilityID,
				ControlID:                  controlID,
				OrganizationID:             statementOfApplicability.OrganizationID,
				Applicability:              applicability,
				Justification:              justification,
				CreatedAt:                  now,
				UpdatedAt:                  now,
			}

			if err := applicabilityStatement.Insert(ctx, conn, s.svc.scope); err != nil {
				return fmt.Errorf("cannot insert applicability statement: %w", err)
			}

			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	return applicabilityStatement, nil
}

func (s StatementOfApplicabilityService) UpdateApplicabilityStatement(
	ctx context.Context,
	applicabilityStatementID gid.GID,
	applicability bool,
	justification *string,
) (*coredata.ApplicabilityStatement, error) {
	applicabilityStatement := &coredata.ApplicabilityStatement{}

	err := s.svc.pg.WithTx(
		ctx,
		func(ctx context.Context, conn pg.Tx) error {
			if err := applicabilityStatement.LoadByID(ctx, conn, s.svc.scope, applicabilityStatementID); err != nil {
				return err
			}

			applicabilityStatement.Applicability = applicability
			applicabilityStatement.Justification = justification
			applicabilityStatement.UpdatedAt = time.Now()

			return applicabilityStatement.UpdateByID(ctx, conn, s.svc.scope)
		},
	)
	if err != nil {
		return nil, err
	}

	return applicabilityStatement, nil
}

func (s StatementOfApplicabilityService) DeleteApplicabilityStatement(
	ctx context.Context,
	applicabilityStatementID gid.GID,
) error {
	applicabilityStatement := &coredata.ApplicabilityStatement{}

	return s.svc.pg.WithTx(
		ctx,
		func(ctx context.Context, conn pg.Tx) error {
			return applicabilityStatement.DeleteByID(ctx, conn, s.svc.scope, applicabilityStatementID)
		},
	)
}

func (s StatementOfApplicabilityService) ListControlLinks(
	ctx context.Context,
	controlID gid.GID,
	cursor *page.Cursor[coredata.ApplicabilityStatementOrderField],
) (*page.Page[*coredata.ApplicabilityStatement, coredata.ApplicabilityStatementOrderField], error) {
	var controls coredata.ApplicabilityStatements

	err := s.svc.pg.WithConn(ctx, func(ctx context.Context, conn pg.Querier) error {
		return controls.LoadByControlID(ctx, conn, s.svc.scope, controlID, cursor)
	})
	if err != nil {
		return nil, err
	}

	return page.NewPage(controls, cursor), nil
}

func (s StatementOfApplicabilityService) buildDocumentData(
	ctx context.Context,
	conn pg.Querier,
	statementOfApplicabilityID gid.GID,
) (docgen.StatementOfApplicabilityData, error) {
	statementOfApplicability := &coredata.StatementOfApplicability{}
	if err := statementOfApplicability.LoadByID(ctx, conn, s.svc.scope, statementOfApplicabilityID); err != nil {
		return docgen.StatementOfApplicabilityData{}, fmt.Errorf("cannot load statement of applicability: %w", err)
	}

	organization := &coredata.Organization{}
	if err := organization.LoadByID(ctx, conn, s.svc.scope, statementOfApplicability.OrganizationID); err != nil {
		return docgen.StatementOfApplicabilityData{}, fmt.Errorf("cannot load organization: %w", err)
	}

	var applicabilityStatements coredata.ApplicabilityStatements
	cursor := page.NewCursor(
		10000,
		nil,
		page.Head,
		page.OrderBy[coredata.ApplicabilityStatementOrderField]{
			Field:     coredata.ApplicabilityStatementOrderFieldControlSectionTitle,
			Direction: page.OrderDirectionAsc,
		},
	)
	if err := applicabilityStatements.LoadByStatementOfApplicabilityID(ctx, conn, s.svc.scope, statementOfApplicabilityID, cursor); err != nil {
		return docgen.StatementOfApplicabilityData{}, fmt.Errorf("cannot load applicability statements: %w", err)
	}

	if len(applicabilityStatements) == 0 {
		return docgen.StatementOfApplicabilityData{
			Title:            statementOfApplicability.Name,
			OrganizationName: organization.Name,
			CreatedAt:        statementOfApplicability.CreatedAt,
			TotalControls:    0,
			FrameworkGroups:  []docgen.FrameworkControlGroup{},
		}, nil
	}

	frameworkControlsMap := make(map[string][]docgen.ControlData)
	frameworkOrder := []string{}

	for _, stmt := range applicabilityStatements {
		control := &coredata.Control{}
		if err := control.LoadByID(ctx, conn, s.svc.scope, stmt.ControlID); err != nil {
			return docgen.StatementOfApplicabilityData{}, fmt.Errorf("cannot load control: %w", err)
		}

		framework := &coredata.Framework{}
		if err := framework.LoadByID(ctx, conn, s.svc.scope, control.FrameworkID); err != nil {
			return docgen.StatementOfApplicabilityData{}, fmt.Errorf("cannot load framework: %w", err)
		}

		var controlObligations coredata.ControlObligations
		legalType := coredata.ObligationTypeLegal
		legalFilter := coredata.NewControlObligationFilter(&legalType)
		legalCount, err := controlObligations.CountByControlID(ctx, conn, s.svc.scope, stmt.ControlID, legalFilter)
		if err != nil {
			return docgen.StatementOfApplicabilityData{}, fmt.Errorf("cannot count legal obligations: %w", err)
		}

		contractualType := coredata.ObligationTypeContractual
		contractualFilter := coredata.NewControlObligationFilter(&contractualType)
		contractualCount, err := controlObligations.CountByControlID(ctx, conn, s.svc.scope, stmt.ControlID, contractualFilter)
		if err != nil {
			return docgen.StatementOfApplicabilityData{}, fmt.Errorf("cannot count contractual obligations: %w", err)
		}

		var controlsWithRisk coredata.ControlsWithRisk
		if err := controlsWithRisk.LoadByControlIDs(ctx, conn, s.svc.scope, []gid.GID{stmt.ControlID}); err != nil {
			return docgen.StatementOfApplicabilityData{}, fmt.Errorf("cannot load controls with risks: %w", err)
		}
		hasRisk := len(controlsWithRisk) > 0

		if _, exists := frameworkControlsMap[framework.Name]; !exists {
			frameworkOrder = append(frameworkOrder, framework.Name)
			frameworkControlsMap[framework.Name] = []docgen.ControlData{}
		}

		var regulatory *bool
		var contractual *bool
		var bestPractice *bool
		var riskAssessment *bool

		if stmt.Applicability {
			falseVal := false
			trueVal := true

			regulatory = &falseVal
			contractual = &falseVal
			riskAssessment = &falseVal

			if legalCount > 0 {
				regulatory = &trueVal
			}
			if contractualCount > 0 {
				contractual = &trueVal
			}
			if hasRisk {
				riskAssessment = &trueVal
			}

			bestPractice = &control.BestPractice
		}

		applicability := stmt.Applicability

		implemented := control.Implemented.String()
		frameworkControlsMap[framework.Name] = append(
			frameworkControlsMap[framework.Name],
			docgen.ControlData{
				FrameworkName: framework.Name,
				SectionTitle:  control.SectionTitle,
				Name:          control.Name,
				Applicability: &applicability,
				Justification: stmt.Justification,
				BestPractice:  bestPractice,
				Implemented:   &implemented,
				NotImplementedJustification: func() *string {
					if control.Implemented == coredata.ControlImplementationStateImplemented {
						return nil
					}
					return control.NotImplementedJustification
				}(),
				Regulatory:     regulatory,
				Contractual:    contractual,
				RiskAssessment: riskAssessment,
			},
		)
	}

	frameworkGroups := make([]docgen.FrameworkControlGroup, len(frameworkOrder))
	for i, frameworkName := range frameworkOrder {
		frameworkGroups[i] = docgen.FrameworkControlGroup{
			FrameworkName: frameworkName,
			Controls:      frameworkControlsMap[frameworkName],
		}
	}

	var snapshots coredata.Snapshots
	snapshotType := coredata.SnapshotsTypeStatementsOfApplicability

	var version int
	var publishedAt time.Time

	if statementOfApplicability.SnapshotID != nil {
		snapshot := &coredata.Snapshot{}
		if err := snapshot.LoadByID(ctx, conn, s.svc.scope, *statementOfApplicability.SnapshotID); err != nil {
			return docgen.StatementOfApplicabilityData{}, fmt.Errorf("cannot load snapshot: %w", err)
		}
		publishedAt = snapshot.CreatedAt
		snapshotFilter := coredata.NewSnapshotFilter(&snapshotType).WithBeforeDate(&snapshot.CreatedAt)
		snapshotCount, err := snapshots.CountByOrganizationID(ctx, conn, s.svc.scope, statementOfApplicability.OrganizationID, snapshotFilter)
		if err != nil {
			return docgen.StatementOfApplicabilityData{}, fmt.Errorf("cannot count states of applicability snapshots: %w", err)
		}
		version = snapshotCount
	} else {
		publishedAt = time.Now()
		snapshotFilter := coredata.NewSnapshotFilter(&snapshotType)
		snapshotCount, err := snapshots.CountByOrganizationID(ctx, conn, s.svc.scope, statementOfApplicability.OrganizationID, snapshotFilter)
		if err != nil {
			return docgen.StatementOfApplicabilityData{}, fmt.Errorf("cannot count states of applicability snapshots: %w", err)
		}
		version = snapshotCount + 1
	}

	horizontalLogoBase64 := ""
	if organization.HorizontalLogoFileID != nil {
		fileRecord := &coredata.File{}
		fileErr := fileRecord.LoadByID(ctx, conn, s.svc.scope, *organization.HorizontalLogoFileID)
		if fileErr == nil {
			base64Data, mimeType, logoErr := s.svc.fileManager.GetFileBase64(ctx, fileRecord)
			if logoErr == nil {
				horizontalLogoBase64 = fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)
			}
		}
	}

	return docgen.StatementOfApplicabilityData{
		Title:                       statementOfApplicability.Name,
		OrganizationName:            organization.Name,
		CreatedAt:                   statementOfApplicability.CreatedAt,
		TotalControls:               len(applicabilityStatements),
		FrameworkGroups:             frameworkGroups,
		CompanyHorizontalLogoBase64: horizontalLogoBase64,
		Version:                     version,
		PublishedAt:                 publishedAt,
		Approver:                    "",
	}, nil
}

func (s StatementOfApplicabilityService) ExportPDF(
	ctx context.Context,
	statementOfApplicabilityID gid.GID,
) ([]byte, error) {
	var documentData docgen.StatementOfApplicabilityData

	err := s.svc.pg.WithConn(
		ctx,
		func(ctx context.Context, conn pg.Querier) error {
			var err error
			documentData, err = s.buildDocumentData(ctx, conn, statementOfApplicabilityID)
			return err
		},
	)

	if err != nil {
		return nil, err
	}

	htmlData, err := docgen.RenderStatementOfApplicabilityHTML(documentData)
	if err != nil {
		return nil, fmt.Errorf("cannot render HTML: %w", err)
	}

	cfg := html2pdf.RenderConfig{
		PageFormat:      html2pdf.PageFormatA4,
		Orientation:     html2pdf.OrientationPortrait,
		MarginTop:       html2pdf.NewMarginInches(1.0),
		MarginBottom:    html2pdf.NewMarginInches(1.0),
		MarginLeft:      html2pdf.NewMarginInches(1.0),
		MarginRight:     html2pdf.NewMarginInches(1.0),
		PrintBackground: true,
		Scale:           1.0,
	}

	pdfReader, err := s.html2pdfConverter.GeneratePDF(ctx, htmlData, cfg)
	if err != nil {
		return nil, fmt.Errorf("cannot generate PDF: %w", err)
	}

	pdfData, err := io.ReadAll(pdfReader)
	if err != nil {
		return nil, fmt.Errorf("cannot read PDF data: %w", err)
	}

	return pdfData, nil
}

func (s StatementOfApplicabilityService) CreateDocument(
	ctx context.Context,
	statementOfApplicabilityID gid.GID,
	approverIDs []gid.GID,
) (*coredata.Document, *coredata.DocumentVersion, error) {
	var document *coredata.Document
	var documentVersion *coredata.DocumentVersion

	err := s.svc.pg.WithTx(
		ctx,
		func(ctx context.Context, tx pg.Tx) error {
			documentData, err := s.buildDocumentData(ctx, tx, statementOfApplicabilityID)
			if err != nil {
				return fmt.Errorf("cannot build document data: %w", err)
			}

			prosemirrorJSON, err := buildSOAProseMirrorDocument(documentData)
			if err != nil {
				return fmt.Errorf("cannot build prosemirror document: %w", err)
			}

			soa := &coredata.StatementOfApplicability{}
			if err := soa.LoadByID(ctx, tx, s.svc.scope, statementOfApplicabilityID); err != nil {
				return fmt.Errorf("cannot load statement of applicability: %w", err)
			}

			now := time.Now()
			landscape := coredata.DocumentVersionOrientationLandscape

			var existingDoc *coredata.Document
			if soa.DocumentID != nil {
				doc := &coredata.Document{}
				err = doc.LoadByID(ctx, tx, s.svc.scope, *soa.DocumentID)
				if err != nil && !errors.Is(err, coredata.ErrResourceNotFound) {
					return fmt.Errorf("cannot load SOA document: %w", err)
				}

				if err == nil && doc.ArchivedAt == nil {
					existingDoc = doc
				} else {
					soa.DocumentID = nil
					soa.UpdatedAt = now
					if err := soa.Update(ctx, tx, s.svc.scope); err != nil {
						return fmt.Errorf("cannot clear SOA document reference: %w", err)
					}
				}
			}

			hasApprovers := len(approverIDs) > 0

			if existingDoc == nil {
				documentID := gid.New(s.svc.scope.GetTenantID(), coredata.DocumentEntityType)

				document = &coredata.Document{
					ID:                    documentID,
					OrganizationID:        soa.OrganizationID,
					TrustCenterVisibility: coredata.TrustCenterVisibilityNone,
					Status:                coredata.DocumentStatusActive,
					CreatedAt:             now,
					UpdatedAt:             now,
				}

				if !hasApprovers {
					document.CurrentPublishedMajor = new(1)
					document.CurrentPublishedMinor = new(0)
				}

				if err := document.Insert(ctx, tx, s.svc.scope); err != nil {
					return fmt.Errorf("cannot insert document: %w", err)
				}

				soa.DocumentID = &documentID
				soa.UpdatedAt = now
				if err := soa.Update(ctx, tx, s.svc.scope); err != nil {
					return fmt.Errorf("cannot update SOA document reference: %w", err)
				}
			} else {
				document = existingDoc

				latestVersion := &coredata.DocumentVersion{}
				if err := latestVersion.LoadLatestVersion(ctx, tx, s.svc.scope, document.ID); err != nil {
					if !errors.Is(err, coredata.ErrResourceNotFound) {
						return fmt.Errorf("cannot load latest version: %w", err)
					}
				} else if latestVersion.Status == coredata.DocumentVersionStatusDraft {
					if err := latestVersion.Delete(ctx, tx, s.svc.scope); err != nil {
						return fmt.Errorf("cannot delete existing draft version: %w", err)
					}
				}
			}

			var newMajor int
			if document.CurrentPublishedMajor != nil {
				newMajor = *document.CurrentPublishedMajor + 1
			} else {
				newMajor = 1
			}

			versionStatus := coredata.DocumentVersionStatusPublished
			var publishedAt *time.Time
			if hasApprovers {
				versionStatus = coredata.DocumentVersionStatusDraft
			} else {
				publishedAt = &now
			}

			documentVersionID := gid.New(s.svc.scope.GetTenantID(), coredata.DocumentVersionEntityType)
			documentVersion = &coredata.DocumentVersion{
				ID:             documentVersionID,
				OrganizationID: soa.OrganizationID,
				DocumentID:     document.ID,
				Title:          soa.Name,
				Major:          newMajor,
				Minor:          0,
				Content:        prosemirrorJSON,
				Status:         versionStatus,
				Classification: coredata.DocumentClassificationConfidential,
				DocumentType:   coredata.DocumentTypeStatementOfApplicability,
				ContentSource:  coredata.DocumentVersionContentSourceGenerated,
				Orientation:    &landscape,
				PublishedAt:    publishedAt,
				CreatedAt:      now,
				UpdatedAt:      now,
			}

			if err := documentVersion.Insert(ctx, tx, s.svc.scope); err != nil {
				return fmt.Errorf("cannot insert document version: %w", err)
			}

			if hasApprovers {
				_, err := s.svc.DocumentApprovals.RequestApprovalInTx(
					ctx,
					tx,
					document,
					documentVersion,
					approverIDs,
					nil,
				)
				if err != nil {
					return fmt.Errorf("cannot request approval: %w", err)
				}
			} else {
				document.CurrentPublishedMajor = &newMajor
				document.CurrentPublishedMinor = new(0)
				document.UpdatedAt = now

				if err := document.Update(ctx, tx, s.svc.scope); err != nil {
					return fmt.Errorf("cannot update document: %w", err)
				}
			}

			return nil
		},
	)

	if err != nil {
		return nil, nil, err
	}

	return document, documentVersion, nil
}

func buildSOAProseMirrorDocument(data docgen.StatementOfApplicabilityData) (string, error) {
	content := []prosemirror.Node{
		pmHeading(1, "1. Purpose"),
		pmParagraph(
			"This document provides a comprehensive overview of the statement of applicability for controls within the organization. " +
				"It serves as a record of which controls are applicable or not applicable to the organization, along with their " +
				"relationships to regulatory requirements, contractual obligations, risk assessments, and best practices.",
		),
	}

	// Column widths in pixels (total ~1100 for landscape A4).
	// Framework 120, Control 250, Applicability 70, Justification 130,
	// Implemented 70, Not-impl justification 110, Reg 60, Contract 60, BP 60, Risk 60.
	colwidths := [][]int{
		{120}, {250}, {70}, {130}, {70}, {110}, {60}, {60}, {60}, {60},
	}

	content = append(content,
		prosemirror.Node{Type: prosemirror.NodeHorizontalRule},
		pmHeading(1, "2. Controls"),
	)

	headerRow1 := prosemirror.Node{
		Type: prosemirror.NodeTableRow,
		Content: []prosemirror.Node{
			pmTableHeaderCellSpanW("Framework", 1, 2, colwidths[0]),
			pmTableHeaderCellSpanW("Control", 1, 2, colwidths[1]),
			pmTableHeaderCellSpanW("Applicability", 1, 2, colwidths[2]),
			pmTableHeaderCellSpanW("Justification for non-applicability", 1, 2, colwidths[3]),
			pmTableHeaderCellSpanW("Implemented", 1, 2, colwidths[4]),
			pmTableHeaderCellSpanW("Justification for non-implementation", 1, 2, colwidths[5]),
			pmTableHeaderCellSpanW("Justification for inclusion", 4, 1, nil),
		},
	}
	headerRow2 := prosemirror.Node{
		Type: prosemirror.NodeTableRow,
		Content: []prosemirror.Node{
			pmTableHeaderCellW("Regulatory", colwidths[6]),
			pmTableHeaderCellW("Contractual", colwidths[7]),
			pmTableHeaderCellW("Best Practice", colwidths[8]),
			pmTableHeaderCellW("Risk Assessment", colwidths[9]),
		},
	}

	rows := []prosemirror.Node{headerRow1, headerRow2}

	for _, group := range data.FrameworkGroups {
		for _, ctrl := range group.Controls {
			justification := "-"
			if ctrl.Applicability != nil && !*ctrl.Applicability && ctrl.Justification != nil {
				justification = *ctrl.Justification
			}

			implemented := "-"
			if ctrl.Implemented != nil {
				if ctrl.Applicability != nil && !*ctrl.Applicability {
					implemented = "-"
				} else if *ctrl.Implemented == "IMPLEMENTED" {
					implemented = "Yes"
				} else {
					implemented = "No"
				}
			}

			notImplJustification := "-"
			if ctrl.Implemented != nil && *ctrl.Implemented == "NOT_IMPLEMENTED" && ctrl.NotImplementedJustification != nil {
				notImplJustification = *ctrl.NotImplementedJustification
			}

			controlCell := pmTableCellW(colwidths[1], pmControlParagraph(ctrl.SectionTitle, ctrl.Name))

			cells := []prosemirror.Node{
				pmTableCellW(colwidths[0], pmParagraph(group.FrameworkName)),
				controlCell,
				pmTableCellW(colwidths[2], pmParagraph(boolLabel(ctrl.Applicability))),
				pmTableCellW(colwidths[3], pmParagraph(justification)),
				pmTableCellW(colwidths[4], pmParagraph(implemented)),
				pmTableCellW(colwidths[5], pmParagraph(notImplJustification)),
				pmTableCellW(colwidths[6], pmParagraph(boolLabel(ctrl.Regulatory))),
				pmTableCellW(colwidths[7], pmParagraph(boolLabel(ctrl.Contractual))),
				pmTableCellW(colwidths[8], pmParagraph(boolLabel(ctrl.BestPractice))),
				pmTableCellW(colwidths[9], pmParagraph(boolLabel(ctrl.RiskAssessment))),
			}

			rows = append(rows, prosemirror.Node{
				Type:    prosemirror.NodeTableRow,
				Content: cells,
			})
		}
	}

	content = append(content, prosemirror.Node{
		Type:    prosemirror.NodeTable,
		Content: rows,
	})

	content = append(content, buildSOAAnnex()...)

	doc := prosemirror.Node{
		Type:    prosemirror.NodeDoc,
		Content: content,
	}

	b, err := json.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("cannot marshal prosemirror document: %w", err)
	}

	return string(b), nil
}

func pmHeading(level int, text string) prosemirror.Node {
	attrs, _ := json.Marshal(prosemirror.HeadingAttrs{Level: level})
	return prosemirror.Node{
		Type:  prosemirror.NodeHeading,
		Attrs: attrs,
		Content: []prosemirror.Node{
			pmText(text),
		},
	}
}

func pmParagraph(text string) prosemirror.Node {
	if text == "" {
		return prosemirror.Node{
			Type: prosemirror.NodeParagraph,
		}
	}
	return prosemirror.Node{
		Type: prosemirror.NodeParagraph,
		Content: []prosemirror.Node{
			pmText(text),
		},
	}
}

func pmText(text string) prosemirror.Node {
	return prosemirror.Node{
		Type: prosemirror.NodeText,
		Text: &text,
	}
}

func pmTableCellW(colwidth []int, content ...prosemirror.Node) prosemirror.Node {
	attrs, _ := json.Marshal(prosemirror.TableCellAttrs{
		Colspan:  1,
		Rowspan:  1,
		Colwidth: colwidth,
	})
	return prosemirror.Node{
		Type:    prosemirror.NodeTableCell,
		Attrs:   attrs,
		Content: content,
	}
}

func pmTableHeaderCellSpanW(text string, colspan, rowspan int, colwidth []int) prosemirror.Node {
	attrs, _ := json.Marshal(prosemirror.TableCellAttrs{
		Colspan:  colspan,
		Rowspan:  rowspan,
		Colwidth: colwidth,
	})
	return prosemirror.Node{
		Type:  prosemirror.NodeTableHeader,
		Attrs: attrs,
		Content: []prosemirror.Node{
			{
				Type: prosemirror.NodeParagraph,
				Content: []prosemirror.Node{
					{
						Type:  prosemirror.NodeText,
						Text:  &text,
						Marks: []prosemirror.Mark{{Type: prosemirror.MarkStrong}},
					},
				},
			},
		},
	}
}

func pmTableHeaderCellW(text string, colwidth []int) prosemirror.Node {
	return pmTableHeaderCellSpanW(text, 1, 1, colwidth)
}

func pmControlParagraph(sectionTitle, name string) prosemirror.Node {
	tag := fmt.Sprintf("[%s] ", sectionTitle)
	return prosemirror.Node{
		Type: prosemirror.NodeParagraph,
		Content: []prosemirror.Node{
			{
				Type:  prosemirror.NodeText,
				Text:  &tag,
				Marks: []prosemirror.Mark{{Type: prosemirror.MarkCode}},
			},
			pmText(name),
		},
	}
}

func buildSOAAnnex() []prosemirror.Node {
	naApplicability := pmBulletList(
		pmBoldTextParagraph("Yes: ", "The control is applicable to the organization."),
		pmBoldTextParagraph("No: ", "The control is not applicable to the organization (with justification provided)."),
	)

	naImplemented := pmBulletList(
		pmBoldTextParagraph("Yes: ", "The control has been implemented by the organization."),
		pmBoldTextParagraph("No: ", "The control has not been implemented (with justification provided)."),
		pmBoldTextParagraph("-: ", "Not applicable (control is not applicable)."),
	)

	naYesNoNA := func(yes, no string) prosemirror.Node {
		return pmBulletList(
			pmBoldTextParagraph("Yes: ", yes),
			pmBoldTextParagraph("No: ", no),
			pmBoldTextParagraph("-: ", "Not applicable (control is not applicable)."),
		)
	}

	return []prosemirror.Node{
		{Type: prosemirror.NodeHorizontalRule},
		pmHeading(1, "3. Annexes"),
		pmHeading(2, "3.1 Column Definitions"),

		pmHeading(3, "Framework"),
		pmParagraph("The name of the compliance framework or standard to which the control belongs (e.g., ISO 27001, SOC 2, GDPR)."),

		pmHeading(3, "Control"),
		pmParagraph("The specific control identifier and name within the framework, including its section reference."),

		pmHeading(3, "Applicability"),
		naApplicability,

		pmHeading(3, "Justification for non-applicability"),
		pmParagraph("Provides the rationale when a control is not applicable. This field is empty for applicable controls."),

		pmHeading(3, "Implemented"),
		naImplemented,

		pmHeading(3, "Justification for non-implementation"),
		pmParagraph("Provides the rationale when a control is not implemented. This field is empty for implemented controls or when the control is not applicable."),

		pmHeading(3, "Justification for inclusion"),
		pmParagraph("For applicable controls, this section provides additional context on why the control is included, based on regulatory requirements, contractual obligations, best practices, or risk assessments."),

		pmHeading(4, "Regulatory"),
		naYesNoNA(
			"The control is linked to one or more legal or regulatory obligations.",
			"The control is not associated with any legal or regulatory obligations.",
		),

		pmHeading(4, "Contractual"),
		naYesNoNA(
			"The control is linked to one or more contractual obligations.",
			"The control is not associated with any contractual obligations.",
		),

		pmHeading(4, "Best Practice"),
		naYesNoNA(
			"The control is designated as a best practice recommendation.",
			"The control is not designated as a best practice.",
		),

		pmHeading(4, "Risk Assessment"),
		naYesNoNA(
			"The control is associated with one or more identified risks through risk mitigation measures.",
			"The control is not currently associated with any identified risks.",
		),
	}
}

func pmBulletList(items ...prosemirror.Node) prosemirror.Node {
	listItems := make([]prosemirror.Node, len(items))
	for i, item := range items {
		listItems[i] = prosemirror.Node{
			Type:    prosemirror.NodeListItem,
			Content: []prosemirror.Node{item},
		}
	}
	return prosemirror.Node{
		Type:    prosemirror.NodeBulletList,
		Content: listItems,
	}
}

func pmBoldTextParagraph(bold, text string) prosemirror.Node {
	return prosemirror.Node{
		Type: prosemirror.NodeParagraph,
		Content: []prosemirror.Node{
			{
				Type:  prosemirror.NodeText,
				Text:  &bold,
				Marks: []prosemirror.Mark{{Type: prosemirror.MarkStrong}},
			},
			pmText(text),
		},
	}
}

func boolLabel(v *bool) string {
	if v == nil {
		return "-"
	}
	if *v {
		return "Yes"
	}
	return "No"
}
