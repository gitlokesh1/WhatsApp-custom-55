# 88Task earning portal deployment

The earning portal initializes its country, reward-snapshot, and banner metadata schema automatically after connecting to PostgreSQL. Existing earning-ledger rows are treated as INR and users with historical earnings are assigned to India.

Banner uploads require these server-side environment variables:

- `SUPABASE_URL`: project API URL
- `SUPABASE_SERVICE_ROLE_KEY`: service role key; never expose this value to browser code
- `SUPABASE_BANNER_BUCKET`: public Supabase Storage bucket name (defaults to `user-banners`)

Create the banner bucket as a public bucket before enabling uploads. The admin endpoint accepts JPEG, PNG, and WebP images up to 5 MB. All other portal features require only the existing `SUPABASE_DB_URL` and `ADMIN_TOKEN` configuration.
