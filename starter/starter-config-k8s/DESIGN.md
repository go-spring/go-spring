# starter-config-k8s Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-config-k8s` is a config-provider starter (`starter/DESIGN.md` §2.5)
in the integration layer: it makes a Kubernetes ConfigMap or Secret a
hot-reloadable configuration source, read directly through the API server via
a client-go informer rather than through a mounted volume. It complements
`starter-config-file`.

## 1. Responsibilities & Boundaries

- Registers a `k8s` provider name via `conf.RegisterProvider` in `init()` and
  nothing else at the package top level — no injectable bean, no server.
- Parses the provider source `k8s:<kind>/<name>?namespace=&key=&format=&kubeconfig=`,
  fetches the object, decodes selected data entries by extension or forced
  format, and merges the flattened result.
- Installs a client-go informer on the object; every add/update/delete event
  fires the application-wide property refresh.

## 2. Key Abstractions & Seams

- **Provider seam.** `conf.RegisterProvider("k8s", loadK8sConfig)`. The
  provider runs during `AppConfig.Refresh`, before any bean exists.
- **`k8sClient` interface.** The provider takes a narrow interface (`CoreV1`
  read access) instead of a `*kubernetes.Clientset` directly, so tests can
  inject a client-go fake clientset without a live cluster. `buildClient`
  resolves the real client from in-cluster config or a kubeconfig file.
- **Informer seam.** One shared informer per `(kind, namespace, name)` triple
  under `ensureWatch`; on any event it calls the refresh hook.
- **Refresh hook.** Container-scope bridge bean, exported as `gs.Rooter` and
  named to avoid the `__default__` collision.

## 3. Constraints

- **Register the informer before returning.** `ensureWatch` runs before the
  provider returns so a change landing between the initial read and the
  informer sync is not missed.
- **Merge `Data` and `BinaryData` for ConfigMaps.** ConfigMap payload lives
  in both fields; the provider merges them into a single `name -> bytes` map
  and treats them uniformly.
- **Unknown entry extensions are skipped in whole-object mode.** A ConfigMap
  often contains non-config entries (`README.md`, template files); the
  provider only parses entries whose extension maps to a known reader. With
  `?key=<one>` a single entry must parse, so an unknown format there is a
  hard error (with a message telling the user to set `format=`).
- **Secret payload is already `[]byte`.** No base64 decode step is needed;
  the client-go objects expose decoded `Data`.
- **RBAC is on the caller.** The provider only calls `Get`/`Watch` on the
  configured kind; the ServiceAccount must have `get,list,watch` for that
  kind in the referenced namespace.

## 4. Trade-offs / Alternatives Rejected

- **A single mega-provider that covers files and API — rejected.** File
  mounts and API access differ in RBAC, failure modes, and latency
  (kubelet's ~1min Secret rotation vs. seconds through the API). Two
  starters keep the mental model and failure modes clean; the app blank-
  imports whichever matches its deployment.
- **Custom watcher instead of `SharedInformerFactory` — rejected.** The
  informer already gives resync, connection retries, and event
  coalescing; hand-rolling those is unwarranted duplication.
