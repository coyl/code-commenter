# Secrets to create in Google Secret Manager

Create these secrets once per GCP project. Then reference them in Cloud Run (e.g. `--set-secrets=GEMINI_API_KEY=code-commenter-gemini-api-key:latest`) or in `deploy/env.prod.yaml` (secret links).

Use the Google Cloud Console or `gcloud`:

```bash
# Set your project
export PROJECT_ID=your-gcp-project-id

# Create secrets (you will be prompted for the secret value, or use --data-file)
gcloud secrets create code-commenter-gemini-api-key --project="$PROJECT_ID"
gcloud secrets create code-commenter-google-client-secret --project="$PROJECT_ID"
gcloud secrets create code-commenter-session-secret --project="$PROJECT_ID"
gcloud secrets create code-commenter-s3-access-key --project="$PROJECT_ID"
gcloud secrets create code-commenter-s3-secret-key --project="$PROJECT_ID"

# Add a version (example: from stdin)
echo -n "your-api-key" | gcloud secrets versions add code-commenter-gemini-api-key --data-file=-
```

| Secret name | Used as env var | Description |
|-------------|-----------------|-------------|
| **code-commenter-gemini-api-key** | `GEMINI_API_KEY` | Required. Gemini/Google AI API key for all agents. |
| **code-commenter-google-client-secret** | `GOOGLE_CLIENT_SECRET` | Optional. OAuth client secret when using Google sign-in. |
| **code-commenter-session-secret** | `SESSION_SECRET` | Optional. At least 32-byte random string for session cookie signing (required when OAuth is enabled). |
| **code-commenter-s3-access-key** | `S3_ACCESS_KEY` | Optional. S3-compatible storage access key for job persistence. |
| **code-commenter-s3-secret-key** | `S3_SECRET_KEY` | Optional. S3-compatible storage secret key. |

Ensure the Cloud Run service account has **Secret Manager Secret Accessor** on these secrets (or on the project).
