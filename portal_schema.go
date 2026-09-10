package main

// initPortalBaseSchema creates the foundational portal tables that the
// feature-specific migrations extend. All statements are additive so an
// existing Supabase deployment keeps its data and schema customizations.
func initPortalBaseSchema() error {
	_, err := userDB.Exec(`
		CREATE TABLE IF NOT EXISTS public.app_users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id TEXT NOT NULL UNIQUE,
			referral_code TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			display_name TEXT NOT NULL,
			referred_by UUID REFERENCES public.app_users(id),
			balance NUMERIC(14,4) NOT NULL DEFAULT 0,
			today_earning NUMERIC(14,4) NOT NULL DEFAULT 0,
			total_earning NUMERIC(14,4) NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'active',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);

		CREATE TABLE IF NOT EXISTS public.user_sessions_auth (
			token_hash TEXT PRIMARY KEY,
			user_id UUID NOT NULL REFERENCES public.app_users(id) ON DELETE CASCADE,
			expires_at TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		ALTER TABLE public.user_sessions_auth ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ NOT NULL DEFAULT (now() + interval '30 days');
		ALTER TABLE public.user_sessions_auth ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();
		CREATE INDEX IF NOT EXISTS user_sessions_auth_user_idx
		ON public.user_sessions_auth(user_id);
		CREATE INDEX IF NOT EXISTS user_sessions_auth_expiry_idx
		ON public.user_sessions_auth(expires_at);

		CREATE TABLE IF NOT EXISTS public.whatsapp_sessions (
			user_id TEXT PRIMARY KEY,
			jid TEXT NOT NULL,
			phone TEXT NOT NULL DEFAULT '',
			state TEXT NOT NULL DEFAULT 'offline',
			connected BOOLEAN NOT NULL DEFAULT false,
			logged_in BOOLEAN NOT NULL DEFAULT false,
			last_seen_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);

		CREATE TABLE IF NOT EXISTS public.user_whatsapp_accounts (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES public.app_users(id) ON DELETE CASCADE,
			whatsapp_user_id TEXT NOT NULL UNIQUE,
			phone_number TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			linked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			last_seen_at TIMESTAMPTZ,
			removed_at TIMESTAMPTZ,
			current_send_total BIGINT NOT NULL DEFAULT 0,
			today_send_total BIGINT NOT NULL DEFAULT 0,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		ALTER TABLE public.user_whatsapp_accounts ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active';
		CREATE INDEX IF NOT EXISTS user_whatsapp_accounts_user_idx
		ON public.user_whatsapp_accounts(user_id,status);

		CREATE TABLE IF NOT EXISTS public.task_definitions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			title TEXT NOT NULL,
			message TEXT NOT NULL,
			target_phone TEXT NOT NULL DEFAULT '',
			reward NUMERIC(14,4) NOT NULL DEFAULT 0,
			active BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		ALTER TABLE public.task_definitions ADD COLUMN IF NOT EXISTS channel TEXT NOT NULL DEFAULT 'whatsapp';

		CREATE TABLE IF NOT EXISTS public.task_claims (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			task_id UUID NOT NULL REFERENCES public.task_definitions(id),
			user_id UUID NOT NULL REFERENCES public.app_users(id),
			whatsapp_account_id UUID NOT NULL REFERENCES public.user_whatsapp_accounts(id),
			target_phone TEXT NOT NULL,
			message TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'claimed',
			reward NUMERIC(14,4) NOT NULL DEFAULT 0,
			sent_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		ALTER TABLE public.task_claims ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'claimed';
		ALTER TABLE public.task_claims ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();
		ALTER TABLE public.task_claims ALTER COLUMN whatsapp_account_id DROP NOT NULL;
		ALTER TABLE public.task_claims ADD COLUMN IF NOT EXISTS channel TEXT NOT NULL DEFAULT 'whatsapp';
		ALTER TABLE public.task_claims ADD COLUMN IF NOT EXISTS android_installation_id UUID;
		ALTER TABLE public.task_claims ADD COLUMN IF NOT EXISTS sms_nonce_hash TEXT;
		ALTER TABLE public.task_claims ADD COLUMN IF NOT EXISTS sms_public_key_der BYTEA;
		ALTER TABLE public.task_claims ADD COLUMN IF NOT EXISTS sms_parts INTEGER;
		ALTER TABLE public.task_claims ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;
		ALTER TABLE public.task_claims ADD COLUMN IF NOT EXISTS failure_reason TEXT NOT NULL DEFAULT '';
		CREATE UNIQUE INDEX IF NOT EXISTS task_claims_active_user_task_idx
		ON public.task_claims(task_id,user_id)
		WHERE status IN ('claimed','sending','sent');
		CREATE INDEX IF NOT EXISTS task_claims_user_idx
		ON public.task_claims(user_id,created_at DESC);

		CREATE TABLE IF NOT EXISTS public.android_installations (
			id UUID PRIMARY KEY,
			user_id UUID NOT NULL REFERENCES public.app_users(id) ON DELETE CASCADE,
			public_key_der BYTEA NOT NULL,
			active BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE INDEX IF NOT EXISTS android_installations_user_idx ON public.android_installations(user_id,active);
		DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='task_claims_android_installation_fk') THEN
				ALTER TABLE public.task_claims ADD CONSTRAINT task_claims_android_installation_fk FOREIGN KEY(android_installation_id) REFERENCES public.android_installations(id);
			END IF;
			IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='task_definitions_channel_check') THEN
				ALTER TABLE public.task_definitions ADD CONSTRAINT task_definitions_channel_check CHECK(channel IN ('whatsapp','sms'));
			END IF;
			IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='task_claims_channel_check') THEN
				ALTER TABLE public.task_claims ADD CONSTRAINT task_claims_channel_check CHECK(channel IN ('whatsapp','sms'));
			END IF;
		END $$;

		CREATE TABLE IF NOT EXISTS public.earning_ledger (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES public.app_users(id),
			task_claim_id UUID REFERENCES public.task_claims(id),
			amount NUMERIC(14,4) NOT NULL,
			type TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);

		CREATE TABLE IF NOT EXISTS public.mlm_settings (
			level INTEGER PRIMARY KEY CHECK(level BETWEEN 1 AND 10),
			commission_percent NUMERIC(7,4) NOT NULL DEFAULT 0 CHECK(commission_percent >= 0),
			active BOOLEAN NOT NULL DEFAULT false,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);

		CREATE TABLE IF NOT EXISTS public.referral_commissions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			referrer_id UUID NOT NULL REFERENCES public.app_users(id),
			referred_user_id UUID NOT NULL REFERENCES public.app_users(id),
			task_claim_id UUID NOT NULL REFERENCES public.task_claims(id),
			amount NUMERIC(14,4) NOT NULL CONSTRAINT referral_commissions_amount_positive CHECK(amount > 0),
			level INTEGER NOT NULL CHECK(level BETWEEN 1 AND 10),
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			UNIQUE(referrer_id,task_claim_id,level)
		);

		CREATE TABLE IF NOT EXISTS public.customer_care_tickets (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES public.app_users(id) ON DELETE CASCADE,
			subject TEXT NOT NULL,
			message TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'open',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		ALTER TABLE public.customer_care_tickets ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();
		CREATE INDEX IF NOT EXISTS customer_care_tickets_user_idx
		ON public.customer_care_tickets(user_id,created_at DESC);

		ALTER TABLE public.app_users ADD COLUMN IF NOT EXISTS referred_by UUID REFERENCES public.app_users(id);
		CREATE INDEX IF NOT EXISTS app_users_referred_by_idx
		ON public.app_users(referred_by) WHERE referred_by IS NOT NULL;
		ALTER TABLE public.app_users ADD COLUMN IF NOT EXISTS balance NUMERIC(14,4) NOT NULL DEFAULT 0;
		ALTER TABLE public.app_users ADD COLUMN IF NOT EXISTS today_earning NUMERIC(14,4) NOT NULL DEFAULT 0;
		ALTER TABLE public.app_users ADD COLUMN IF NOT EXISTS total_earning NUMERIC(14,4) NOT NULL DEFAULT 0;
		ALTER TABLE public.app_users ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active';
		ALTER TABLE public.app_users ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();
		ALTER TABLE public.app_users ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

		ALTER TABLE public.whatsapp_sessions ADD COLUMN IF NOT EXISTS jid TEXT NOT NULL DEFAULT '';
		ALTER TABLE public.whatsapp_sessions ADD COLUMN IF NOT EXISTS phone TEXT NOT NULL DEFAULT '';
		ALTER TABLE public.whatsapp_sessions ADD COLUMN IF NOT EXISTS state TEXT NOT NULL DEFAULT 'offline';
		ALTER TABLE public.whatsapp_sessions ADD COLUMN IF NOT EXISTS connected BOOLEAN NOT NULL DEFAULT false;
		ALTER TABLE public.whatsapp_sessions ADD COLUMN IF NOT EXISTS logged_in BOOLEAN NOT NULL DEFAULT false;
		ALTER TABLE public.whatsapp_sessions ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ;
		ALTER TABLE public.whatsapp_sessions ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

		ALTER TABLE public.user_whatsapp_accounts ADD COLUMN IF NOT EXISTS phone_number TEXT NOT NULL DEFAULT '';
		ALTER TABLE public.user_whatsapp_accounts ADD COLUMN IF NOT EXISTS linked_at TIMESTAMPTZ NOT NULL DEFAULT now();
		ALTER TABLE public.user_whatsapp_accounts ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ;
		ALTER TABLE public.user_whatsapp_accounts ADD COLUMN IF NOT EXISTS removed_at TIMESTAMPTZ;
		ALTER TABLE public.user_whatsapp_accounts ADD COLUMN IF NOT EXISTS current_send_total BIGINT NOT NULL DEFAULT 0;
		ALTER TABLE public.user_whatsapp_accounts ADD COLUMN IF NOT EXISTS today_send_total BIGINT NOT NULL DEFAULT 0;
		ALTER TABLE public.user_whatsapp_accounts ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

		ALTER TABLE public.task_definitions ADD COLUMN IF NOT EXISTS target_phone TEXT NOT NULL DEFAULT '';
		ALTER TABLE public.task_definitions ADD COLUMN IF NOT EXISTS reward NUMERIC(14,4) NOT NULL DEFAULT 0;
		ALTER TABLE public.task_definitions ADD COLUMN IF NOT EXISTS active BOOLEAN NOT NULL DEFAULT true;
		ALTER TABLE public.task_definitions ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();
		ALTER TABLE public.task_definitions ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

		ALTER TABLE public.task_claims ADD COLUMN IF NOT EXISTS target_phone TEXT NOT NULL DEFAULT '';
		ALTER TABLE public.task_claims ADD COLUMN IF NOT EXISTS message TEXT NOT NULL DEFAULT '';
		ALTER TABLE public.task_claims ADD COLUMN IF NOT EXISTS reward NUMERIC(14,4) NOT NULL DEFAULT 0;
		ALTER TABLE public.task_claims ADD COLUMN IF NOT EXISTS sent_at TIMESTAMPTZ;
		ALTER TABLE public.task_claims ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

		ALTER TABLE public.earning_ledger ADD COLUMN IF NOT EXISTS task_claim_id UUID REFERENCES public.task_claims(id);
		ALTER TABLE public.earning_ledger ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
		ALTER TABLE public.earning_ledger ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();

		ALTER TABLE public.mlm_settings ADD COLUMN IF NOT EXISTS commission_percent NUMERIC(7,4) NOT NULL DEFAULT 0;
		ALTER TABLE public.mlm_settings ADD COLUMN IF NOT EXISTS active BOOLEAN NOT NULL DEFAULT false;
		ALTER TABLE public.mlm_settings ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

		ALTER TABLE public.referral_commissions ADD COLUMN IF NOT EXISTS amount NUMERIC(14,4) NOT NULL DEFAULT 0;
		DO $$ BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint
				WHERE conrelid='public.referral_commissions'::regclass
				AND conname='referral_commissions_amount_positive'
			) THEN
				ALTER TABLE public.referral_commissions
				ADD CONSTRAINT referral_commissions_amount_positive CHECK(amount > 0) NOT VALID;
			END IF;
		END $$;
		ALTER TABLE public.referral_commissions ADD COLUMN IF NOT EXISTS level INTEGER NOT NULL DEFAULT 1;
		ALTER TABLE public.referral_commissions ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();

		ALTER TABLE public.customer_care_tickets ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'open';
		ALTER TABLE public.customer_care_tickets ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
	`)
	return err
}
