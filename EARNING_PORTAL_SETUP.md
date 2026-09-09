# 88Task earning portal deployment

The earning portal initializes its country, wallet, withdrawal, and banner schema automatically after connecting to PostgreSQL. Existing earning rows are migrated idempotently into the signed wallet ledger without changing user balances.

## Countries

Open **Admin → Earning portal → Countries & rates → Add country**. Search the preset catalog, select a country, review its currency/timezone preview, set the reward and daily goal, and save it. Only saved database countries appear in registration, tasks, banners, users, and withdrawals. Enable a country's withdrawal switch only after at least one payout channel has been configured for it.

Banner uploads require these server-side environment variables:

- `SUPABASE_URL`: project API URL
- `SUPABASE_SERVICE_ROLE_KEY`: service role key; never expose this value to browser code
- `SUPABASE_BANNER_BUCKET`: public Supabase Storage bucket name (defaults to `user-banners`)

Create the banner bucket as a public bucket before enabling uploads. The admin endpoint accepts JPEG, PNG, and WebP images up to 5 MB.

## Payout gateway

Withdrawals remain visibly disabled until all three server-only values are present:

- `PAYOUT_GATEWAY_URL`: HTTPS endpoint that accepts an approved payout job
- `PAYOUT_GATEWAY_SECRET`: high-entropy HMAC secret shared with the gateway
- `PAYOUT_DATA_ENCRYPTION_KEY`: high-entropy key used to encrypt beneficiary details at rest

The gateway must verify `X-88Task-Timestamp`, `X-88Task-Signature`, and `Idempotency-Key`. It sends signed callbacks to `POST /api/payouts/webhook` using the same timestamp/signature format. After setting the secrets, configure country channels in **Admin → Withdrawals**, enable the global switch, and then enable the relevant country and individual users.

All other portal features require the existing `SUPABASE_DB_URL` and `ADMIN_TOKEN` configuration. Never expose service, payout, database, or admin secrets to browser code.
