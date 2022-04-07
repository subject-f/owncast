-- Queries added to query.sql must be compiled into Go code with sqlc. Read README.md for details.

-- Federation related queries.

-- name: GetFollowerCount :one
SElECT count(*) FROM ap_followers WHERE approved_at is not null;

-- name: GetLocalPostCount :one
SElECT count(*) FROM ap_outbox;

-- name: GetFederationFollowersWithOffset :many
SELECT iri, inbox, name, username, image, created_at FROM ap_followers WHERE approved_at is not null ORDER BY created_at DESC LIMIT $1 OFFSET $2;

-- name: GetRejectedAndBlockedFollowers :many
SELECT iri, name, username, image, created_at, disabled_at FROM ap_followers WHERE disabled_at is not null;

-- name: GetFederationFollowerApprovalRequests :many
SELECT iri, inbox, name, username, image, created_at FROM ap_followers WHERE approved_at IS null AND disabled_at is null;

-- name: ApproveFederationFollower :exec
UPDATE ap_followers SET approved_at = $1, disabled_at = null WHERE iri = $2;

-- name: RejectFederationFollower :exec
UPDATE ap_followers SET approved_at = null, disabled_at = $1 WHERE iri = $2;

-- name: GetFollowerByIRI :one
SELECT iri, inbox, name, username, image, request, created_at, approved_at, disabled_at FROM ap_followers WHERE iri = $1;

-- name: GetOutboxWithOffset :many
SELECT value FROM ap_outbox LIMIT $1 OFFSET $2;

-- name: GetObjectFromOutboxByID :one
SELECT value FROM ap_outbox WHERE iri = $1;

-- name: GetObjectFromOutboxByIRI :one
SELECT value, live_notification, created_at FROM ap_outbox WHERE iri = $1;

-- name: RemoveFollowerByIRI :exec
DELETE FROM ap_followers WHERE iri = $1;

-- name: AddFollower :exec
INSERT INTO ap_followers(iri, inbox, request, name, username, image, approved_at) values($1, $2, $3, $4, $5, $6, $7);

-- name: AddToOutbox :exec
INSERT INTO ap_outbox(iri, value, type, live_notification) values($1, $2, $3, $4);

-- name: AddToAcceptedActivities :exec
INSERT INTO ap_accepted_activities(iri, actor, type, timestamp) values($1, $2, $3, $4);

-- name: GetInboundActivityCount :one
SELECT count(*) FROM ap_accepted_activities;

-- name: GetInboundActivitiesWithOffset :many
SELECT iri, actor, type, timestamp FROM ap_accepted_activities ORDER BY timestamp DESC LIMIT $1 OFFSET $2;

-- name: DoesInboundActivityExist :one
SELECT count(*) FROM ap_accepted_activities WHERE iri = $1 AND actor = $2 AND TYPE = $3;

-- name: UpdateFollowerByIRI :exec
UPDATE ap_followers SET inbox = $1, name = $2, username = $3, image = $4 WHERE iri = $5;

-- name: BanIPAddress :exec
INSERT INTO ip_bans(ip_address, notes) values($1, $2);

-- name: RemoveIPAddressBan :exec
DELETE FROM ip_bans WHERE ip_address = $1;

-- name: IsIPAddressBlocked :one
SELECT count(*) FROM ip_bans WHERE ip_address = $1;

-- name: GetIPAddressBans :many
SELECT * FROM ip_bans;
-- name: AddNotification :exec
INSERT INTO notifications (channel, destination) VALUES($1, $2);

-- name: GetNotificationDestinationsForChannel :many
SELECT destination FROM notifications WHERE channel = $1;

-- name: RemoveNotificationDestinationForChannel :exec
DELETE FROM notifications WHERE channel = $1 AND destination = $2;
