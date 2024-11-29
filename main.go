package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/paymentintent"
)

const addr = "localhost:4242"

func main() {
	// This is your test secret API key.
	stripe.Key = os.Getenv("API_KEY")

	http.HandleFunc("/checkout", checkout)
	http.HandleFunc("/success", success)
	http.HandleFunc("/payment_intent/{id}", paymentIntent)
	http.HandleFunc("/webhook", webhook)

	slog.Info("server started", slog.String("addr", addr))

	if err := http.ListenAndServe(addr, nil); err != nil {
		slog.Error("http.ListenAndServe", slog.Any("err", err))
	}
}

func checkout(w http.ResponseWriter, r *http.Request) {
	domain := "http://" + addr

	params := &stripe.CheckoutSessionParams{
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				// Provide the exact Price ID (for example, pr_1234) of the product you want to sell
				Price:    stripe.String(os.Getenv("PRICE_ID")),
				Quantity: stripe.Int64(1),
			},
		},
		Mode:       stripe.String(string(stripe.CheckoutSessionModePayment)),
		SuccessURL: stripe.String(domain + "/success?session_id={CHECKOUT_SESSION_ID}"),
		CancelURL:  stripe.String(domain + "/cancel"),
		Metadata: map[string]string{ // metadata for the checkout session (doesnt stick throughout the lifecycle)
			"some_unique_id": "some_unique_value",
		},
		PaymentIntentData: &stripe.CheckoutSessionPaymentIntentDataParams{
			Metadata: map[string]string{ // metadata for the payment intent
				"some_unique_id": "some_unique_value",
			},
		},
		ClientReferenceID: stripe.String("MY-CUSTOMER-ID"),
	}

	s, err := session.New(params)
	if err != nil {
		log.Printf("session.New: %v", err)
	}

	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		http.Error(w, "error marshalling JSON", http.StatusInternalServerError)
		return
	}

	slog.Info("checkout session", slog.String("session", string(b)))

	http.Redirect(w, r, s.URL, http.StatusSeeOther)
}

func paymentIntent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	paymentIntent, err := paymentintent.Get(id, nil)
	if err != nil {
		slog.Error("paymentintent.Get", slog.Any("err", err))
		http.Error(w, "error retrieving payment intent", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(paymentIntent)
}

func success(w http.ResponseWriter, r *http.Request) {
	// get session id from query string
	sessionID := r.URL.Query().Get("session_id")

	// retrieve session information from stripe
	session, err := session.Get(sessionID, nil)
	if err != nil {
		slog.Error("session.Get", slog.Any("err", err))

		fmt.Fprintf(w, "Error retrieving session information: %s", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(session)
}

func webhook(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 65536)
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("error reading request body", slog.Any("err", err))

		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	var event stripe.Event

	if err := json.Unmarshal(payload, &event); err != nil {
		slog.Error("error unmarshaling event", slog.Any("err", err))

		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Unmarshal the event data into an appropriate struct depending on its Type
	switch event.Type {
	case stripe.EventTypePaymentIntentSucceeded:
		var paymentIntent stripe.PaymentIntent
		err := json.Unmarshal(event.Data.Raw, &paymentIntent)
		if err != nil {
			slog.Error("error unmarshaling event data", slog.Any("err", err))
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Then define and call a func to handle the successful payment intent.
		// handlePaymentIntentSucceeded(paymentIntent)
	case stripe.EventTypeRefundCreated:
		var refund stripe.Refund
		err := json.Unmarshal(event.Data.Raw, &refund)
		if err != nil {
			slog.Error("error unmarshaling event data", slog.Any("err", err))
			w.WriteHeader(http.StatusBadRequest)
			return
		}

	// ... handle other event types
	default:
		slog.Warn("Unhandled event type", slog.String("event_type", string(event.Type)))
	}

	w.WriteHeader(http.StatusOK)
}
