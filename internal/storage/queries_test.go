//go:build integration

package storage_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sungwon/smtp-proxy/internal/storage"
)

// --- Account Tests ---

func TestCreateAccount(t *testing.T) {
	_, queries := setupTestDB(t)
	ctx := context.Background()

	domains, _ := json.Marshal([]string{"example.com"})
	account, err := queries.CreateAccount(ctx, storage.CreateAccountParams{
		Name:           "test-account",
		Email:          "test@example.com",
		PasswordHash:   "$2a$10$hashhere",
		AllowedDomains: domains,
		ApiKey:         "test-api-key-" + uuid.New().String()[:8],
	})
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}

	if account.Name != "test-account" {
		t.Errorf("expected name 'test-account', got %s", account.Name)
	}
	if account.Email != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got %s", account.Email)
	}
	if account.ID == uuid.Nil {
		t.Error("expected non-nil UUID for account ID")
	}
}

func TestGetAccountByID(t *testing.T) {
	_, queries := setupTestDB(t)
	ctx := context.Background()

	domains, _ := json.Marshal([]string{"example.com"})
	created, err := queries.CreateAccount(ctx, storage.CreateAccountParams{
		Name:           "lookup-test",
		Email:          "lookup@example.com",
		PasswordHash:   "$2a$10$hashhere",
		AllowedDomains: domains,
		ApiKey:         "lookup-key-" + uuid.New().String()[:8],
	})
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}

	fetched, err := queries.GetAccountByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetAccountByID failed: %v", err)
	}

	if fetched.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, fetched.ID)
	}
	if fetched.Name != "lookup-test" {
		t.Errorf("expected name 'lookup-test', got %s", fetched.Name)
	}
}

func TestGetAccountByAPIKey(t *testing.T) {
	_, queries := setupTestDB(t)
	ctx := context.Background()

	apiKey := "apikey-" + uuid.New().String()[:8]
	domains, _ := json.Marshal([]string{})
	_, err := queries.CreateAccount(ctx, storage.CreateAccountParams{
		Name:           "apikey-test",
		Email:          "api@example.com",
		PasswordHash:   "$2a$10$hashhere",
		AllowedDomains: domains,
		ApiKey:         apiKey,
	})
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}

	fetched, err := queries.GetAccountByAPIKey(ctx, apiKey)
	if err != nil {
		t.Fatalf("GetAccountByAPIKey failed: %v", err)
	}

	if fetched.ApiKey != apiKey {
		t.Errorf("expected api_key %s, got %s", apiKey, fetched.ApiKey)
	}
}

func TestGetAccountByName(t *testing.T) {
	_, queries := setupTestDB(t)
	ctx := context.Background()

	domains, _ := json.Marshal([]string{})
	_, err := queries.CreateAccount(ctx, storage.CreateAccountParams{
		Name:           "name-lookup-test",
		Email:          "name@example.com",
		PasswordHash:   "$2a$10$hashhere",
		AllowedDomains: domains,
		ApiKey:         "namekey-" + uuid.New().String()[:8],
	})
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}

	fetched, err := queries.GetAccountByName(ctx, "name-lookup-test")
	if err != nil {
		t.Fatalf("GetAccountByName failed: %v", err)
	}

	if fetched.Name != "name-lookup-test" {
		t.Errorf("expected name 'name-lookup-test', got %s", fetched.Name)
	}
}

func TestUpdateAccount(t *testing.T) {
	_, queries := setupTestDB(t)
	ctx := context.Background()

	domains, _ := json.Marshal([]string{})
	created, err := queries.CreateAccount(ctx, storage.CreateAccountParams{
		Name:           "update-test",
		Email:          "old@example.com",
		PasswordHash:   "$2a$10$hashhere",
		AllowedDomains: domains,
		ApiKey:         "updatekey-" + uuid.New().String()[:8],
	})
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}

	newDomains, _ := json.Marshal([]string{"new.com"})
	updated, err := queries.UpdateAccount(ctx, storage.UpdateAccountParams{
		ID:             created.ID,
		Name:           "updated-name",
		Email:          "new@example.com",
		AllowedDomains: newDomains,
	})
	if err != nil {
		t.Fatalf("UpdateAccount failed: %v", err)
	}

	if updated.Name != "updated-name" {
		t.Errorf("expected name 'updated-name', got %s", updated.Name)
	}
	if updated.Email != "new@example.com" {
		t.Errorf("expected email 'new@example.com', got %s", updated.Email)
	}
}

func TestDeleteAccount(t *testing.T) {
	_, queries := setupTestDB(t)
	ctx := context.Background()

	domains, _ := json.Marshal([]string{})
	created, err := queries.CreateAccount(ctx, storage.CreateAccountParams{
		Name:           "delete-test",
		Email:          "delete@example.com",
		PasswordHash:   "$2a$10$hashhere",
		AllowedDomains: domains,
		ApiKey:         "deletekey-" + uuid.New().String()[:8],
	})
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}

	err = queries.DeleteAccount(ctx, created.ID)
	if err != nil {
		t.Fatalf("DeleteAccount failed: %v", err)
	}

	_, err = queries.GetAccountByID(ctx, created.ID)
	if err == nil {
		t.Error("expected error when getting deleted account")
	}
}

func TestListAccounts(t *testing.T) {
	_, queries := setupTestDB(t)
	ctx := context.Background()

	domains, _ := json.Marshal([]string{})
	for i := 0; i < 3; i++ {
		_, err := queries.CreateAccount(ctx, storage.CreateAccountParams{
			Name:           "list-test-" + uuid.New().String()[:8],
			Email:          "list@example.com",
			PasswordHash:   "$2a$10$hashhere",
			AllowedDomains: domains,
			ApiKey:         "listkey-" + uuid.New().String()[:8],
		})
		if err != nil {
			t.Fatalf("CreateAccount failed: %v", err)
		}
	}

	accounts, err := queries.ListAccounts(ctx)
	if err != nil {
		t.Fatalf("ListAccounts failed: %v", err)
	}

	if len(accounts) < 3 {
		t.Errorf("expected at least 3 accounts, got %d", len(accounts))
	}
}

// --- Provider Tests ---

func createTestAccount(t *testing.T, queries *storage.Queries) storage.Account {
	t.Helper()
	ctx := context.Background()
	domains, _ := json.Marshal([]string{"example.com"})
	account, err := queries.CreateAccount(ctx, storage.CreateAccountParams{
		Name:           "provider-test-" + uuid.New().String()[:8],
		Email:          "provider@example.com",
		PasswordHash:   "$2a$10$hashhere",
		AllowedDomains: domains,
		ApiKey:         "provkey-" + uuid.New().String()[:8],
	})
	if err != nil {
		t.Fatalf("createTestAccount failed: %v", err)
	}
	return account
}

func TestCreateProvider(t *testing.T) {
	_, queries := setupTestDB(t)
	ctx := context.Background()
	account := createTestAccount(t, queries)

	smtpConfig, _ := json.Marshal(map[string]string{"host": "smtp.example.com"})
	provider, err := queries.CreateProvider(ctx, storage.CreateProviderParams{
		AccountID:    account.ID,
		Name:         "test-sendgrid",
		ProviderType: storage.ProviderTypeSendgrid,
		ApiKey:       sql.NullString{String: "sg-key-123", Valid: true},
		SmtpConfig:   smtpConfig,
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("CreateProvider failed: %v", err)
	}

	if provider.Name != "test-sendgrid" {
		t.Errorf("expected name 'test-sendgrid', got %s", provider.Name)
	}
	if provider.ProviderType != storage.ProviderTypeSendgrid {
		t.Errorf("expected provider type 'sendgrid', got %s", provider.ProviderType)
	}
	if !provider.Enabled {
		t.Error("expected provider to be enabled")
	}
}

func TestGetProviderByID(t *testing.T) {
	_, queries := setupTestDB(t)
	ctx := context.Background()
	account := createTestAccount(t, queries)

	created, err := queries.CreateProvider(ctx, storage.CreateProviderParams{
		AccountID:    account.ID,
		Name:         "get-provider",
		ProviderType: storage.ProviderTypeMailgun,
		ApiKey:       sql.NullString{String: "mg-key", Valid: true},
		SmtpConfig:   nil,
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("CreateProvider failed: %v", err)
	}

	fetched, err := queries.GetProviderByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetProviderByID failed: %v", err)
	}

	if fetched.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, fetched.ID)
	}
}

func TestListProvidersByAccountID(t *testing.T) {
	_, queries := setupTestDB(t)
	ctx := context.Background()
	account := createTestAccount(t, queries)

	for i := 0; i < 2; i++ {
		_, err := queries.CreateProvider(ctx, storage.CreateProviderParams{
			AccountID:    account.ID,
			Name:         "list-provider-" + uuid.New().String()[:8],
			ProviderType: storage.ProviderTypeSes,
			ApiKey:       sql.NullString{String: "ses-key", Valid: true},
			SmtpConfig:   nil,
			Enabled:      true,
		})
		if err != nil {
			t.Fatalf("CreateProvider failed: %v", err)
		}
	}

	providers, err := queries.ListProvidersByAccountID(ctx, account.ID)
	if err != nil {
		t.Fatalf("ListProvidersByAccountID failed: %v", err)
	}

	if len(providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(providers))
	}
}

func TestUpdateProvider(t *testing.T) {
	_, queries := setupTestDB(t)
	ctx := context.Background()
	account := createTestAccount(t, queries)

	created, err := queries.CreateProvider(ctx, storage.CreateProviderParams{
		AccountID:    account.ID,
		Name:         "old-provider",
		ProviderType: storage.ProviderTypeSmtp,
		ApiKey:       sql.NullString{Valid: false},
		SmtpConfig:   nil,
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("CreateProvider failed: %v", err)
	}

	updated, err := queries.UpdateProvider(ctx, storage.UpdateProviderParams{
		ID:           created.ID,
		Name:         "new-provider",
		ProviderType: storage.ProviderTypeMsgraph,
		ApiKey:       sql.NullString{String: "new-key", Valid: true},
		SmtpConfig:   nil,
		Enabled:      false,
	})
	if err != nil {
		t.Fatalf("UpdateProvider failed: %v", err)
	}

	if updated.Name != "new-provider" {
		t.Errorf("expected name 'new-provider', got %s", updated.Name)
	}
	if updated.Enabled {
		t.Error("expected provider to be disabled")
	}
}

func TestDeleteProvider(t *testing.T) {
	_, queries := setupTestDB(t)
	ctx := context.Background()
	account := createTestAccount(t, queries)

	created, err := queries.CreateProvider(ctx, storage.CreateProviderParams{
		AccountID:    account.ID,
		Name:         "delete-provider",
		ProviderType: storage.ProviderTypeSendgrid,
		ApiKey:       sql.NullString{String: "del-key", Valid: true},
		SmtpConfig:   nil,
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("CreateProvider failed: %v", err)
	}

	err = queries.DeleteProvider(ctx, created.ID)
	if err != nil {
		t.Fatalf("DeleteProvider failed: %v", err)
	}

	_, err = queries.GetProviderByID(ctx, created.ID)
	if err == nil {
		t.Error("expected error when getting deleted provider")
	}
}

// --- Routing Rule Tests ---

func createTestProvider(t *testing.T, queries *storage.Queries, accountID uuid.UUID) storage.EspProvider {
	t.Helper()
	ctx := context.Background()
	provider, err := queries.CreateProvider(ctx, storage.CreateProviderParams{
		AccountID:    accountID,
		Name:         "rule-provider-" + uuid.New().String()[:8],
		ProviderType: storage.ProviderTypeSendgrid,
		ApiKey:       sql.NullString{String: "rule-key", Valid: true},
		SmtpConfig:   nil,
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("createTestProvider failed: %v", err)
	}
	return provider
}

func TestCreateRoutingRule(t *testing.T) {
	_, queries := setupTestDB(t)
	ctx := context.Background()
	account := createTestAccount(t, queries)
	provider := createTestProvider(t, queries, account.ID)

	conditions, _ := json.Marshal(map[string]string{"domain": "example.com"})
	rule, err := queries.CreateRoutingRule(ctx, storage.CreateRoutingRuleParams{
		AccountID:  account.ID,
		Priority:   10,
		Conditions: conditions,
		ProviderID: provider.ID,
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("CreateRoutingRule failed: %v", err)
	}

	if rule.Priority != 10 {
		t.Errorf("expected priority 10, got %d", rule.Priority)
	}
	if rule.ProviderID != provider.ID {
		t.Errorf("expected provider ID %s, got %s", provider.ID, rule.ProviderID)
	}
}

func TestGetRoutingRuleByID(t *testing.T) {
	_, queries := setupTestDB(t)
	ctx := context.Background()
	account := createTestAccount(t, queries)
	provider := createTestProvider(t, queries, account.ID)

	conditions, _ := json.Marshal(map[string]string{})
	created, err := queries.CreateRoutingRule(ctx, storage.CreateRoutingRuleParams{
		AccountID:  account.ID,
		Priority:   5,
		Conditions: conditions,
		ProviderID: provider.ID,
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("CreateRoutingRule failed: %v", err)
	}

	fetched, err := queries.GetRoutingRuleByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetRoutingRuleByID failed: %v", err)
	}

	if fetched.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, fetched.ID)
	}
}

func TestListRoutingRulesByAccountID_OrderedByPriority(t *testing.T) {
	_, queries := setupTestDB(t)
	ctx := context.Background()
	account := createTestAccount(t, queries)
	provider := createTestProvider(t, queries, account.ID)

	conditions, _ := json.Marshal(map[string]string{})
	for _, priority := range []int32{30, 10, 20} {
		_, err := queries.CreateRoutingRule(ctx, storage.CreateRoutingRuleParams{
			AccountID:  account.ID,
			Priority:   priority,
			Conditions: conditions,
			ProviderID: provider.ID,
			Enabled:    true,
		})
		if err != nil {
			t.Fatalf("CreateRoutingRule failed: %v", err)
		}
	}

	rules, err := queries.ListRoutingRulesByAccountID(ctx, account.ID)
	if err != nil {
		t.Fatalf("ListRoutingRulesByAccountID failed: %v", err)
	}

	if len(rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(rules))
	}

	// Should be ordered by priority ASC
	if rules[0].Priority != 10 {
		t.Errorf("expected first rule priority 10, got %d", rules[0].Priority)
	}
	if rules[1].Priority != 20 {
		t.Errorf("expected second rule priority 20, got %d", rules[1].Priority)
	}
	if rules[2].Priority != 30 {
		t.Errorf("expected third rule priority 30, got %d", rules[2].Priority)
	}
}

func TestUpdateRoutingRule(t *testing.T) {
	_, queries := setupTestDB(t)
	ctx := context.Background()
	account := createTestAccount(t, queries)
	provider := createTestProvider(t, queries, account.ID)

	conditions, _ := json.Marshal(map[string]string{})
	created, err := queries.CreateRoutingRule(ctx, storage.CreateRoutingRuleParams{
		AccountID:  account.ID,
		Priority:   1,
		Conditions: conditions,
		ProviderID: provider.ID,
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("CreateRoutingRule failed: %v", err)
	}

	newConditions, _ := json.Marshal(map[string]string{"from": "admin@example.com"})
	updated, err := queries.UpdateRoutingRule(ctx, storage.UpdateRoutingRuleParams{
		ID:         created.ID,
		Priority:   99,
		Conditions: newConditions,
		ProviderID: provider.ID,
		Enabled:    false,
	})
	if err != nil {
		t.Fatalf("UpdateRoutingRule failed: %v", err)
	}

	if updated.Priority != 99 {
		t.Errorf("expected priority 99, got %d", updated.Priority)
	}
	if updated.Enabled {
		t.Error("expected rule to be disabled")
	}
}

func TestDeleteRoutingRule(t *testing.T) {
	_, queries := setupTestDB(t)
	ctx := context.Background()
	account := createTestAccount(t, queries)
	provider := createTestProvider(t, queries, account.ID)

	conditions, _ := json.Marshal(map[string]string{})
	created, err := queries.CreateRoutingRule(ctx, storage.CreateRoutingRuleParams{
		AccountID:  account.ID,
		Priority:   1,
		Conditions: conditions,
		ProviderID: provider.ID,
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("CreateRoutingRule failed: %v", err)
	}

	err = queries.DeleteRoutingRule(ctx, created.ID)
	if err != nil {
		t.Fatalf("DeleteRoutingRule failed: %v", err)
	}

	_, err = queries.GetRoutingRuleByID(ctx, created.ID)
	if err == nil {
		t.Error("expected error when getting deleted routing rule")
	}
}

// --- Message Tests ---

func TestEnqueueMessage(t *testing.T) {
	_, queries := setupTestDB(t)
	ctx := context.Background()
	account := createTestAccount(t, queries)

	recipients, _ := json.Marshal([]string{"to@example.com"})
	headers, _ := json.Marshal(map[string]string{"X-Custom": "value"})
	msg, err := queries.EnqueueMessage(ctx, storage.EnqueueMessageParams{
		AccountID:  account.ID,
		Sender:     "from@example.com",
		Recipients: recipients,
		Subject:    sql.NullString{String: "Test Subject", Valid: true},
		Headers:    headers,
		Body:       "Hello, World!",
	})
	if err != nil {
		t.Fatalf("EnqueueMessage failed: %v", err)
	}

	if msg.Status != storage.MessageStatusQueued {
		t.Errorf("expected status 'queued', got %s", msg.Status)
	}
	if msg.Sender != "from@example.com" {
		t.Errorf("expected sender 'from@example.com', got %s", msg.Sender)
	}
	if msg.Body != "Hello, World!" {
		t.Errorf("expected body 'Hello, World!', got %s", msg.Body)
	}
}

func TestGetMessageByID(t *testing.T) {
	_, queries := setupTestDB(t)
	ctx := context.Background()
	account := createTestAccount(t, queries)

	recipients, _ := json.Marshal([]string{"to@example.com"})
	created, err := queries.EnqueueMessage(ctx, storage.EnqueueMessageParams{
		AccountID:  account.ID,
		Sender:     "from@example.com",
		Recipients: recipients,
		Subject:    sql.NullString{String: "Test", Valid: true},
		Headers:    nil,
		Body:       "Body",
	})
	if err != nil {
		t.Fatalf("EnqueueMessage failed: %v", err)
	}

	fetched, err := queries.GetMessageByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetMessageByID failed: %v", err)
	}

	if fetched.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, fetched.ID)
	}
}

func TestListMessagesByAccountID(t *testing.T) {
	_, queries := setupTestDB(t)
	ctx := context.Background()
	account := createTestAccount(t, queries)

	recipients, _ := json.Marshal([]string{"to@example.com"})
	for i := 0; i < 5; i++ {
		_, err := queries.EnqueueMessage(ctx, storage.EnqueueMessageParams{
			AccountID:  account.ID,
			Sender:     "from@example.com",
			Recipients: recipients,
			Subject:    sql.NullString{String: "Test", Valid: true},
			Headers:    nil,
			Body:       "Body",
		})
		if err != nil {
			t.Fatalf("EnqueueMessage failed: %v", err)
		}
	}

	messages, err := queries.ListMessagesByAccountID(ctx, storage.ListMessagesByAccountIDParams{
		AccountID: account.ID,
		Limit:     3,
	})
	if err != nil {
		t.Fatalf("ListMessagesByAccountID failed: %v", err)
	}

	if len(messages) != 3 {
		t.Errorf("expected 3 messages (limit), got %d", len(messages))
	}
}

func TestUpdateMessageStatus(t *testing.T) {
	_, queries := setupTestDB(t)
	ctx := context.Background()
	account := createTestAccount(t, queries)

	recipients, _ := json.Marshal([]string{"to@example.com"})
	msg, err := queries.EnqueueMessage(ctx, storage.EnqueueMessageParams{
		AccountID:  account.ID,
		Sender:     "from@example.com",
		Recipients: recipients,
		Subject:    sql.NullString{String: "Test", Valid: true},
		Headers:    nil,
		Body:       "Body",
	})
	if err != nil {
		t.Fatalf("EnqueueMessage failed: %v", err)
	}

	err = queries.UpdateMessageStatus(ctx, storage.UpdateMessageStatusParams{
		ID:     msg.ID,
		Status: storage.MessageStatusDelivered,
	})
	if err != nil {
		t.Fatalf("UpdateMessageStatus failed: %v", err)
	}

	updated, err := queries.GetMessageByID(ctx, msg.ID)
	if err != nil {
		t.Fatalf("GetMessageByID failed: %v", err)
	}

	if updated.Status != storage.MessageStatusDelivered {
		t.Errorf("expected status 'delivered', got %s", updated.Status)
	}
	if !updated.ProcessedAt.Valid {
		t.Error("expected processed_at to be set")
	}
}

func TestGetQueuedMessages(t *testing.T) {
	_, queries := setupTestDB(t)
	ctx := context.Background()
	account := createTestAccount(t, queries)

	recipients, _ := json.Marshal([]string{"to@example.com"})
	// Create 3 queued messages
	for i := 0; i < 3; i++ {
		_, err := queries.EnqueueMessage(ctx, storage.EnqueueMessageParams{
			AccountID:  account.ID,
			Sender:     "from@example.com",
			Recipients: recipients,
			Subject:    sql.NullString{String: "Queued", Valid: true},
			Headers:    nil,
			Body:       "Body",
		})
		if err != nil {
			t.Fatalf("EnqueueMessage failed: %v", err)
		}
	}

	// Mark one as delivered
	msgs, _ := queries.ListMessagesByAccountID(ctx, storage.ListMessagesByAccountIDParams{
		AccountID: account.ID,
		Limit:     1,
	})
	if len(msgs) > 0 {
		_ = queries.UpdateMessageStatus(ctx, storage.UpdateMessageStatusParams{
			ID:     msgs[0].ID,
			Status: storage.MessageStatusDelivered,
		})
	}

	queued, err := queries.GetQueuedMessages(ctx, 10)
	if err != nil {
		t.Fatalf("GetQueuedMessages failed: %v", err)
	}

	// Should have at least 2 queued (one was marked delivered)
	if len(queued) < 2 {
		t.Errorf("expected at least 2 queued messages, got %d", len(queued))
	}

	for _, m := range queued {
		if m.Status != storage.MessageStatusQueued {
			t.Errorf("expected all messages to be 'queued', got %s", m.Status)
		}
	}
}

// --- Delivery Log Tests ---

func TestCreateDeliveryLog(t *testing.T) {
	_, queries := setupTestDB(t)
	ctx := context.Background()
	account := createTestAccount(t, queries)
	provider := createTestProvider(t, queries, account.ID)

	recipients, _ := json.Marshal([]string{"to@example.com"})
	msg, err := queries.EnqueueMessage(ctx, storage.EnqueueMessageParams{
		AccountID:  account.ID,
		Sender:     "from@example.com",
		Recipients: recipients,
		Subject:    sql.NullString{String: "Test", Valid: true},
		Headers:    nil,
		Body:       "Body",
	})
	if err != nil {
		t.Fatalf("EnqueueMessage failed: %v", err)
	}

	log, err := queries.CreateDeliveryLog(ctx, storage.CreateDeliveryLogParams{
		MessageID:    msg.ID,
		ProviderID:   provider.ID,
		Status:       "delivered",
		ResponseCode: pgtype.Int4{Int32: 200, Valid: true},
		ResponseBody: pgtype.Text{String: "OK", Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateDeliveryLog failed: %v", err)
	}

	if log.Status != "delivered" {
		t.Errorf("expected status 'delivered', got %s", log.Status)
	}
	if log.ResponseCode.Int32 != 200 {
		t.Errorf("expected response code 200, got %d", log.ResponseCode.Int32)
	}
}

func TestListDeliveryLogsByMessageID(t *testing.T) {
	_, queries := setupTestDB(t)
	ctx := context.Background()
	account := createTestAccount(t, queries)
	provider := createTestProvider(t, queries, account.ID)

	recipients, _ := json.Marshal([]string{"to@example.com"})
	msg, err := queries.EnqueueMessage(ctx, storage.EnqueueMessageParams{
		AccountID:  account.ID,
		Sender:     "from@example.com",
		Recipients: recipients,
		Subject:    sql.NullString{String: "Test", Valid: true},
		Headers:    nil,
		Body:       "Body",
	})
	if err != nil {
		t.Fatalf("EnqueueMessage failed: %v", err)
	}

	// Create multiple delivery logs
	for _, status := range []string{"attempted", "failed", "delivered"} {
		_, err := queries.CreateDeliveryLog(ctx, storage.CreateDeliveryLogParams{
			MessageID:    msg.ID,
			ProviderID:   provider.ID,
			Status:       status,
			ResponseCode: pgtype.Int4{Int32: 200, Valid: true},
			ResponseBody: pgtype.Text{String: "OK", Valid: true},
		})
		if err != nil {
			t.Fatalf("CreateDeliveryLog failed: %v", err)
		}
	}

	logs, err := queries.ListDeliveryLogsByMessageID(ctx, msg.ID)
	if err != nil {
		t.Fatalf("ListDeliveryLogsByMessageID failed: %v", err)
	}

	if len(logs) != 3 {
		t.Errorf("expected 3 delivery logs, got %d", len(logs))
	}
}
