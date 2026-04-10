-- Copyright (c) 2026 Probo Inc <hello@getprobo.com>.
--
-- Permission to use, copy, modify, and/or distribute this software for any
-- purpose with or without fee is hereby granted, provided that the above
-- copyright notice and this permission notice appear in all copies.
--
-- THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES WITH
-- REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY
-- AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY SPECIAL, DIRECT,
-- INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM
-- LOSS OF USE, DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR
-- OTHER TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR
-- PERFORMANCE OF THIS SOFTWARE.

CREATE TABLE statement_of_applicability_default_approvers (
    statement_of_applicability_id text                     NOT NULL,
    approver_profile_id           text                     NOT NULL,
    tenant_id                     text                     NOT NULL,
    organization_id               text                     NOT NULL,
    created_at                    timestamp with time zone NOT NULL,
    updated_at                    timestamp with time zone NOT NULL,
    PRIMARY KEY (statement_of_applicability_id, approver_profile_id),
    FOREIGN KEY (statement_of_applicability_id) REFERENCES statements_of_applicability(id) ON UPDATE CASCADE ON DELETE CASCADE,
    FOREIGN KEY (approver_profile_id) REFERENCES iam_membership_profiles(id) ON UPDATE CASCADE ON DELETE CASCADE
);
