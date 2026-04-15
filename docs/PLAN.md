# PLAN: Publicar v0.2.0 — API pública de DaemonConfig

## Contexto

El commit `e61e296` (9 abril 2026) introdujo una refactorización provider-agnostic que **cambió el nombre de los campos públicos de `DaemonConfig`**. Este cambio no fue acompañado de un nuevo tag semver. El último tag publicado sigue siendo `v0.1.0` (22 febrero 2026), que tiene el API vieja.

### El break concreto

| Campo en `v0.1.0` (publicado) | Campo en HEAD local (sin publicar) |
|---|---|
| `CmdEdgeWorkerDir` | `EdgeDir` |
| `DeployEdgeWorkerDir` | `OutputDir` |

`tinywasm/app` apunta a `v0.1.0` en `go.mod`, pero `app/section-deploy.go` ya usa los nombres nuevos. El compilador ve el struct del tag publicado → **`unknown field EdgeDir`**, **`unknown field OutputDir`**.

## Por qué cambió la API

El refactor introdujo una interfaz `Provider` para desacoplar el módulo de `goflare` directamente. Antes, `DaemonConfig` exponía rutas con los nombres de la herramienta interna (`CmdEdgeWorkerDir`, `DeployEdgeWorkerDir`). La nueva API los simplifica a nombres agnósticos del proveedor (`EdgeDir`, `OutputDir`), coherentes con la abstracción `Provider`.

El cambio es justificado: los nombres anteriores filtraban un detalle de implementación (la CLI de goflare) hacia el consumidor del módulo.

## Solución

Publicar `v0.2.0` con el API actual del HEAD.

### Pasos

1. Asegurarse de que el HEAD compila y los tests pasan:
   ```bash
   cd tinywasm/deploy
   go build ./...
   go test ./...
   ```

2. Crear y publicar el tag:
   ```bash
   git tag v0.2.0
   git push origin v0.2.0
   ```

3. Actualizar `tinywasm/app`:
   ```bash
   cd tinywasm/app
   go get github.com/tinywasm/deploy@v0.2.0
   go mod tidy
   ```

4. Verificar que `app` compila sin errores:
   ```bash
   go build ./...
   ```

## Checklist de validación

- [ ] `deploy` HEAD compila (`go build ./...`)
- [ ] Tests de deploy pasan (`go test ./...`)
- [ ] Tag `v0.2.0` publicado en remote
- [ ] `app/go.mod` apunta a `v0.2.0`
- [ ] `app` compila sin `unknown field` errors
