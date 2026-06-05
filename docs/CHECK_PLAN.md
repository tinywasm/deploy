# PLAN — `tinywasm/deploy`: usar `update.Swap`/`update.Rollback` (swap ya movido)

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.
> Zero-context agent: todo lo necesario está aquí.

**Breaking change. Sin duplicación, sin legacy, SRP estricto.** La lógica de swap/backup/
restore (pasos 7–10 de `HandleUpdate`) **ya fue movida** a `github.com/tinywasm/update` como
`Swap`/`Rollback` (preserva la lógica `os.Rename` con backup `".old"` de este módulo). Aquí solo
queda **reconectar** `HandleUpdate` para llamar a `update.*` y borrar el código inline duplicado.
El concepto "dual purpose" se elimina.

---

## Development Rules (MANDATORY)

- Comentarios y docs en **inglés**.
- **SRP**: deploy orquesta despliegue (webhook, HMAC, descarga autenticada, ciclo de proceso,
  health checks). El swap atómico NO es su responsabilidad → es de `update`.
- **Sin duplicación / sin legacy**: borra el swap inline y el artefacto `app-failed.exe`; borra
  `docs/DUAL_PURPOSE_SUPPORT.md`.
- El `Downloader` autenticado (PAT de GitHub) **se queda** en deploy (es específico de despliegue;
  `update` solo descarga assets públicos).
- Sin nuevas deps externas más allá de `github.com/tinywasm/update`.

## Dependencia

`go get github.com/tinywasm/update && go mod tidy`. API usada:
```go
update.Swap(targetPath, newFilePath string) (backup string, err error) // backup="" si target no existía
update.Rollback(targetPath, backupPath string) error
```

## Stage 1 — Reconectar `handler.go` (`HandleUpdate`)

El `tempFile` ya se descarga arriba (paso 5). Sustituye los pasos 7–10 (backup/move/rollback
inline) por:

```go
appPath := filepath.Join(app.Path, app.Executable)

// 7+8. Swap atómico (update es dueño del backup/restore). tempFile ya descargado.
backupPath, err := update.Swap(appPath, tempFile)
if err != nil {
    http.Error(w, fmt.Sprintf("Failed to install: %v", err), http.StatusInternalServerError)
    return
}

// 9. Arrancar el nuevo proceso; si falla, rollback al binario anterior.
if err := h.Process.Start(appPath); err != nil {
    _ = update.Rollback(appPath, backupPath)
    _ = h.Process.Start(appPath)
    http.Error(w, fmt.Sprintf("Failed to start: %v", err), http.StatusInternalServerError)
    return
}

// 10. Health-check; rollback si no está sano.
if app.StartupDelay > 0 {
    time.Sleep(app.StartupDelay)
}
newStatus, err := h.Checker.Check(app.HealthEndpoint)
if err != nil || newStatus.Status != "ok" {
    _ = h.Process.Stop(app.Executable)
    _ = update.Rollback(appPath, backupPath)
    _ = h.Process.Start(appPath)
    http.Error(w, "New version failed health check", http.StatusInternalServerError)
    return
}

// 11. Persistir versión (sin cambios respecto al original).
if req.Tag != "" {
    app.Version = req.Tag
    if h.ConfigPath != "" {
        if data, err := yaml.Marshal(h.Config); err == nil {
            _ = os.WriteFile(h.ConfigPath, data, 0644)
        }
    }
}
w.WriteHeader(http.StatusOK)
w.Write([]byte("Update successful"))
```

Borra: el `os.Stat`/`os.Rename` de backup, el `os.Rename(tempFile, appPath)` inline, y todo el
manejo de `app-failed.exe`. Añade el import `"github.com/tinywasm/update"`. Mantén `os` (se usa
en `os.WriteFile` del paso 11) y los demás imports en uso.

## Stage 2 — Borrar el documento dual-purpose y limpiar índices

- **Borra `docs/DUAL_PURPOSE_SUPPORT.md`**.
- Quita todo enlace a él desde `README.md` y otros docs (`docs/ARQUITECTURE_DESIGN.md`,
  `docs/IMPLEMENTATION_GUIDE.md`).
- En `docs/ARQUITECTURE_DESIGN.md` añade un párrafo: el reemplazo atómico de binario
  (backup/swap/rollback) se delega en `github.com/tinywasm/update` (`Swap`/`Rollback`); deploy
  solo orquesta descarga, ciclo de proceso y health checks.

## Stage 3 — Tests

- `go test ./...`. Si algún test asume artefactos viejos (`*.old` con nombre propio,
  `app-failed.exe`) o el rename inline, actualízalo: tras update OK el binario nuevo está en
  `appPath`; tras fallo de start/health el binario previo se restaura en `appPath`.
- No dupliques un test del mecanismo de swap: su contrato está en `update` (`TestSwapAndRollback`,
  `TestSwapFreshInstall`).

---

## Stages table

| Stage | Output | Done when |
|------|--------|-----------|
| 1 | `handler.go` | `HandleUpdate` usa `update.Swap`/`update.Rollback`; swap inline y `app-failed.exe` borrados; compila |
| 2 | `docs/` + `README.md` | `DUAL_PURPOSE_SUPPORT.md` borrado; sin enlaces colgando; ARCHITECTURE nota la delegación |
| 3 | tests | `go test ./...` verde |

## Acceptance criteria

- `go test ./...` verde; `go vet ./...` limpio.
- `handler.go` sin lógica de `os.Rename`/backup propia — solo `update.Swap`/`update.Rollback`.
- `DUAL_PURPOSE_SUPPORT.md` no existe ni se referencia.
- El `Downloader` autenticado queda intacto.
