// Package silon is the official Go SDK for the Silon API.
//
// Construct a client with functional options; configuration falls back to
// the SILON_API_KEY, SILON_BASE_URL and SILON_WORKSPACE environment
// variables:
//
//	client, err := silon.NewClient(
//		silon.WithAPIKey("sk_live_..."),
//		silon.WithWorkspace("acme"), // => https://acme.silon.tech
//	)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	sent, err := client.Messages.Send(ctx, silon.MessageSendParams{
//		Channel: "whatsapp",
//		To:      map[string]any{"client_id": "cust_001"},
//		Content: map[string]any{"body": "Your order has shipped"},
//	})
//
// Every operation takes a context.Context as its first argument. API
// failures are returned as *APIError (inspect with errors.As or the
// Is* predicate helpers such as IsNotFound); transport failures as
// *ConnectionError; configuration, validation and parse failures as the
// base *Error.
//
// The SDK retries idempotent requests (GET/HEAD/OPTIONS/PUT/DELETE, plus
// any request carrying an Idempotency-Key header) on connection errors and
// HTTP 429/500/502/503/504, with exponential backoff that honours the
// server's Retry-After / RateLimit-Reset hints. Messages.Send and OTP.Send
// always send an Idempotency-Key (auto-generated UUIDv4 when not
// supplied), so their retries can never double-send.
//
// Webhook deliveries are verified without an HTTP client via
// VerifyWebhookSignature and ConstructWebhookEvent.
package silon
