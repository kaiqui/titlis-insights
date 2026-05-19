# CLAUDE.md — titlis-insights

> Leia o CLAUDE.md raiz antes de qualquer alteração.

## O que é este serviço

`titlis-insights` é o **único serviço com credenciais Datadog** na plataforma Titlis.
Abstrai fontes de dados externas e expõe um motor unificado de recomendações de HPA.

**Porta:** 8091 (configurável via `PORT`)

## Responsabilidades

- Abstrair fontes de dados externas (Datadog v1 hoje; Prometheus/CloudWatch no roadmap)
- Motor de recomendações HPA: P95 CPU + replica peak + headroom + arredondamento
- Templates de HPA por (tenant, env, criticality) para ambientes sem Datadog
- Verificação de credenciais Datadog (`/v1/datadog/probe`)
- Única superfície autorizada a falar com `api.datadoghq.com`

## O que NÃO é responsabilidade

- Avaliar regras de scorecard → `titlis-scoreops`
- Criar PRs ou acessar GitHub → `titlis-prbot`
- Acessar Kubernetes → `titlis-operator-go`
- Persistir campaigns → `titlis-api`

## Endpoints

| Método | Rota | Auth |
|---|---|---|
| GET | `/v1/recommendations/hpa` | `X-Internal-Secret` |
| GET | `/v1/tenants/{id}/hpa-templates` | `X-Internal-Secret` |
| PUT | `/v1/tenants/{id}/hpa-templates` | `X-Internal-Secret` |
| GET | `/v1/datadog/probe?tenant_id=...` | `X-Internal-Secret` |
| GET | `/health` | público |

## Fontes de métricas (`internal/source/`)

| Tipo | Classe | Uso |
|---|---|---|
| Datadog live | `datadog.Client` | prod/preprod com credencial |
| Memory stub | `memory.Source` | local dev e testes |

Fonte ativa: `INSIGHTS_USE_STUB_SOURCE=true` → memory, `false` → Datadog.

## Motor de recomendação (`internal/recommend/`)

```
ForWorkload(ctx, WorkloadContext) → HpaRecommendation
  1. UsageSnapshot do MetricsSource (Datadog ou stub)
  2. Se ErrSourceUnavailable → fallback para TemplateRepo
  3. Se sem template → source="skipped", confidence=0
  4. Aplica headroom (30%), arredonda para potência de 2, cap: min≥1, max≤100
  5. Confidence derivada de: samples/expected_samples (288/day)
```

## Repos de dados

| Variável | Templates | Log |
|---|---|---|
| `DATABASE_URL` vazio | `MemoryTemplateRepo` | `NoopRecommendationLog` |
| `DATABASE_URL` setado | `PGTemplateRepo` | `PGRecommendationLog` |

Schema PostgreSQL: `titlis_insights.*` (ver docs/titlis-prbot-arquitetura.md §12)

## Variáveis de ambiente principais

```
PORT=8091
TITLIS_APP_ENV=local|preprod|prod
INSIGHTS_INTERNAL_SECRET=<secret>
TITLIS_API_BASE_URL=http://titlis-api:8080
TITLIS_API_INTERNAL_SECRET=<secret>
DATADOG_SITE=datadoghq.com
DATADOG_DEFAULT_WINDOW_DAYS=30
DATADOG_MIN_CONFIDENCE=0.7
INSIGHTS_RECOMMENDATION_CACHE_TTL_MINUTES=360
DATABASE_URL=postgres://...         # vazio → memory repos
INSIGHTS_USE_STUB_SOURCE=true       # false em prod
```

## Padrões obrigatórios

1. **Credenciais DD nunca em logs.** `DD-API-KEY` e `DD-APPLICATION-KEY` são headers
   — nunca logar o valor desses headers.
2. **source="skipped" não é erro.** Significa "sem dados suficientes para recomendar".
   `titlis-scoreops` deve tratar como `PERF-004 skipped`, não como falha.
3. **Datadog rate limits.** `queryTimeSeries` trata `429` com `ErrSourceUnavailable`.
   O caller (`recommend/hpa.go`) faz fallback para template se disponível.
4. **Multi-tenant.** Todo SQL filtra por `tenant_id`. Credenciais DD são por tenant
   (`CredentialProvider.ForTenant`).
5. **Never-reduce não se aplica aqui.** A recomendação diz o que SERIA o ideal.
   A validação never-reduce é responsabilidade do `titlis-prbot/ValidatePatch`.

## Como rodar localmente

```bash
cd titlis-insights
INSIGHTS_USE_STUB_SOURCE=true go run ./cmd/insights/
```

## Build e test

```bash
go build -buildvcs=false ./...
go test ./...
```
