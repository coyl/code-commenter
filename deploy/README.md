# Deploy configs (Cloud Run)

This directory holds **Cloud Run–oriented configs** for production-style deployments (images from Artifact Registry, secrets from Secret Manager). For a fast staging deploy from source, use the scripts in `scripts/` instead.

## Files

| File | Purpose |
|------|---------|
| **env.prod.yaml** | Stub env vars and Secret Manager secret names. No real values; replace stubs and reference secrets when deploying. |
| **secrets-to-create.md** | List of secrets to create in Google Secret Manager; create once per project. |

**Secrets:** Any YAML here that holds actual secret values must be **encrypted with [SOPS](https://github.com/getsops/sops)**. Decrypt with `sops -d deploy/<file>.yaml` before use.

## Using env.prod.yaml

1. Copy or edit `env.prod.yaml`: replace `PROJECT_NUMBER` and `REGION` in `ALLOWED_ORIGINS` and `AUTH_CALLBACK_URL`, and set non-secret env (e.g. `S3_BUCKET`, `FIRESTORE_PROJECT_ID`).
2. Create secrets in Secret Manager as listed in **secrets-to-create.md**.
3. For `gcloud run deploy --env-vars-file`, use only the key-value pairs (no top-level `env:` key). You can copy the `env` block from `env.prod.yaml` into a temporary file and remove the `env:` line so the root is key-value pairs.

   Deploy API with a flattened env file and secrets, for example:

   ```bash
   gcloud run deploy code-commenter-api \
     --image REGION-docker.pkg.dev/PROJECT_ID/code-commenter/code-commenter-api:TAG \
     --region REGION \
     --env-vars-file /path/to/flat-env.yaml \
     --set-secrets=GEMINI_API_KEY=code-commenter-gemini-api-key:latest,SESSION_SECRET=code-commenter-session-secret:latest
   ```

   (Repeat for any other vars listed under `secrets` in `env.prod.yaml`.)

Images are produced by the GitHub Actions workflow (see root README, “GitHub Actions (build & push images)”).
