# BBGO-ACC-002 execution 01

Expected-red capture only; reviewer acceptance pending.
Local path prefixes are labeled in this report; raw logs/metadata retain exact values.

Artifacts: `modern/dist/acc002/red-20260917T181655552699Z` (retained locally).
Filesystem: `ext2/ext3`. Source unchanged: `True`.

## Captured commands

### go-version (capture script)

```json
{
  "name": "go-version",
  "argv": ["<go-launcher>", "version"],
  "returncode": 1,
  "note": "GOSUMDB=off caused toolchain verification failure in capture script environment"
}
```

### expected-red (direct execution)

Command: `go test ./accountstore -count=1`
Exit: 1. **Expected red confirmed — compile failure on undefined contract names.**

```
# github.com/larslarsen/bb-go/modern/accountstore [github.com/larslarsen/bb-go/modern/accountstore.test]
accountstore/fuzz_test.go:37:57: undefined: MaxStoredRecords
accountstore/fuzz_test.go:65:23: undefined: MaxSnapshotBytes
accountstore/fuzz_test.go:66:27: undefined: MaxSnapshotBytes
accountstore/fuzz_test.go:74:17: undefined: Open
accountstore/fuzz_test.go:79:23: undefined: ErrCorrupt
accountstore/fuzz_test.go:105:59: undefined: Store
accountstore/persistence_test.go:53:65: undefined: Store
accountstore/persistence_test.go:65:55: undefined: Store
accountstore/persistence_test.go:89:16: undefined: Create
accountstore/persistence_test.go:484:44: undefined: Store
accountstore/persistence_test.go:89:16: too many errors
FAIL    github.com/larslarsen/bb-go/modern/accountstore [build failed]
FAIL
```

**Production files absent:** `types.go`, `store.go`, `codec.go`. Tests reference the
frozen public contract (Store, Create, Open, MaxStoredRecords, MaxSnapshotBytes,
ErrCorrupt). This is the expected missing-implementation red.

## Metadata

Full environment/tool identities and before/after pins are retained in `metadata.json`.

## Review 03 automated capture

Review 02 qualifies the earlier red capture. This section preserves this run only.
Local paths are labeled here; raw artifacts retain exact values.

Artifacts: `modern/dist/acc002/green-20260917T192111797366Z`.
Metadata SHA-256: `9486b54bdcba586516f5102f7afc093864a3ce795d6d117e225296eaa061c19d`.

```json
{
  "started_utc": "2026-09-17T19:21:11.903285+00:00",
  "filesystem": "ext4",
  "environment": {
    "HOME": "<home>",
    "LANG": "C.UTF-8",
    "PATH": "<home>/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.linux-amd64/bin:<home>/go/bin:/usr/bin:/bin",
    "GOWORK": "off",
    "GOTOOLCHAIN": "local",
    "GOENV": "off",
    "GOPROXY": "off",
    "GOSUMDB": "off",
    "GOFLAGS": "-mod=readonly -p=2",
    "GOMAXPROCS": "2",
    "CGO_ENABLED": "1",
    "GOTELEMETRY": "off",
    "GOMODCACHE": "<home>/go/pkg/mod",
    "GOCACHE": "<repo>/modern/dist/acc002/green-20260917T192111797366Z/gocache",
    "TMPDIR": "<repo>/modern/dist/acc002/green-20260917T192111797366Z/tmp",
    "XDG_CACHE_HOME": "<repo>/modern/dist/acc002/green-20260917T192111797366Z/xdg-cache",
    "PYTHONDONTWRITEBYTECODE": "1"
  },
  "tools": {
    "<home>/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.linux-amd64/bin/go": "1db869c560a193573a71be466a34e0d4abb7792d78165c6102cdda069276a3a8",
    "<home>/go/bin/gosec": "eb00a1fb095b161a48c5bcadbe1e246bbafe270da497a122d2e63ade346954c2",
    "<home>/go/bin/govulncheck": "6c92f0536311f5e2083a839c75558e3fb986758a320a402aa8f524c85ffd7400",
    "<repo>/scripts/govulncheck_policy.py": "709cb00d44c62ef6e2d394f457407183d6fb90bc98958c80db0261607bc3c77c"
  },
  "before": {
    "modern/accountauth/types.go": "ac264e0d967543f915d8a305eda7a78e470dcb6146b2048305fc10070fe98dfb",
    "modern/accountauth/records.go": "cafbb6b14c7fab54df0d14e9cf5822c85f8d0d9e5a98c2aa68c56ea705aa0640",
    "modern/accountauth/state.go": "b1807c69ffaf155dd2a4fd8f1ba54ba58bdaa85c2519c84adaabecd8ee48bff1",
    "modern/accountauth/records_test.go": "264aa1b384ce201dc5d69f92488d2ec0807de59ef048b46118f0f96608dd8fe9",
    "modern/accountauth/state_test.go": "949066a95bd9a8cda7b0bcedbbd3bea25f36185f9b16db6f48849638dbeef9ed",
    "modern/accountauth/fuzz_test.go": "08245bcfdc8d2620f0499c69c2eacdd2033c47beec68959b080bd2d9f960b833",
    "modern/go.mod": "1150b94372852355beaffa7104430a21e8f8aa6ec4877bad18d0bcdb71453783",
    "modern/go.sum": "4c91209822dccd4a60955ddd6b8b94a327e88b55721577494c953a705395b83a",
    "modern/network/open.go": "96bf07274832c57ef67ace9a6e0a5dc06651bc7a5b27eef7cff7f480610d4f3b",
    "modern/accountstore/store_test.go": "6c7c3b4c0849670fc4b1527091b0fb40e26854db10013813c885f5cff5940e32",
    "modern/accountstore/persistence_test.go": "25e8308df9ff6a6aa3d1e43747a6d9a09efd66c12d36a6c3e19e1ef3db9e4c3a",
    "modern/accountstore/fuzz_test.go": "ecff630ee50e23b4651d34aab7190f4872b2f8ad2e0d91b718e31d3518bcc9af",
    "modern/accountstore/types.go": "d10523e39dc4b0e40b80be7105094c1df0d514dd3e156509e47de9a753f7b24f",
    "modern/accountstore/store.go": "7bd1a76b1055bcaa1723f751c702aa70fd4e6d40a6947267c3c1a6c1ab9b6d81",
    "modern/accountstore/codec.go": "fe864efcf92f2f2168da37fa7ea3b482b35f0463e438023dad27a10470e0deab"
  },
  "commands": [
    {
      "name": "go-version",
      "argv": [
        "<home>/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.linux-amd64/bin/go",
        "version"
      ],
      "cwd": "<repo>/modern",
      "started_utc": "2026-09-17T19:21:11.903317+00:00",
      "returncode": 0,
      "finished_utc": "2026-09-17T19:21:11.911014+00:00",
      "stdout_sha256": "76227025cc0bc2be7067aa45d11e09cacfd49c58f498f4c2e4f6a9872a607bf9",
      "stderr_sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
      "after": {
        "modern/accountauth/types.go": "ac264e0d967543f915d8a305eda7a78e470dcb6146b2048305fc10070fe98dfb",
        "modern/accountauth/records.go": "cafbb6b14c7fab54df0d14e9cf5822c85f8d0d9e5a98c2aa68c56ea705aa0640",
        "modern/accountauth/state.go": "b1807c69ffaf155dd2a4fd8f1ba54ba58bdaa85c2519c84adaabecd8ee48bff1",
        "modern/accountauth/records_test.go": "264aa1b384ce201dc5d69f92488d2ec0807de59ef048b46118f0f96608dd8fe9",
        "modern/accountauth/state_test.go": "949066a95bd9a8cda7b0bcedbbd3bea25f36185f9b16db6f48849638dbeef9ed",
        "modern/accountauth/fuzz_test.go": "08245bcfdc8d2620f0499c69c2eacdd2033c47beec68959b080bd2d9f960b833",
        "modern/go.mod": "1150b94372852355beaffa7104430a21e8f8aa6ec4877bad18d0bcdb71453783",
        "modern/go.sum": "4c91209822dccd4a60955ddd6b8b94a327e88b55721577494c953a705395b83a",
        "modern/network/open.go": "96bf07274832c57ef67ace9a6e0a5dc06651bc7a5b27eef7cff7f480610d4f3b",
        "modern/accountstore/store_test.go": "6c7c3b4c0849670fc4b1527091b0fb40e26854db10013813c885f5cff5940e32",
        "modern/accountstore/persistence_test.go": "25e8308df9ff6a6aa3d1e43747a6d9a09efd66c12d36a6c3e19e1ef3db9e4c3a",
        "modern/accountstore/fuzz_test.go": "ecff630ee50e23b4651d34aab7190f4872b2f8ad2e0d91b718e31d3518bcc9af",
        "modern/accountstore/types.go": "d10523e39dc4b0e40b80be7105094c1df0d514dd3e156509e47de9a753f7b24f",
        "modern/accountstore/store.go": "7bd1a76b1055bcaa1723f751c702aa70fd4e6d40a6947267c3c1a6c1ab9b6d81",
        "modern/accountstore/codec.go": "fe864efcf92f2f2168da37fa7ea3b482b35f0463e438023dad27a10470e0deab"
      }
    },
    {
      "name": "scanner-identities",
      "argv": [
        "<home>/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.linux-amd64/bin/go",
        "version",
        "-m",
        "<home>/go/bin/gosec",
        "<home>/go/bin/govulncheck"
      ],
      "cwd": "<repo>/modern",
      "started_utc": "2026-09-17T19:21:11.911636+00:00",
      "returncode": 0,
      "finished_utc": "2026-09-17T19:21:11.915153+00:00",
      "stdout_sha256": "d9a59be6752c7fa194dc5e5b5040cf7e3e2686589f11a28c189485384dafe3e1",
      "stderr_sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
      "after": {
        "modern/accountauth/types.go": "ac264e0d967543f915d8a305eda7a78e470dcb6146b2048305fc10070fe98dfb",
        "modern/accountauth/records.go": "cafbb6b14c7fab54df0d14e9cf5822c85f8d0d9e5a98c2aa68c56ea705aa0640",
        "modern/accountauth/state.go": "b1807c69ffaf155dd2a4fd8f1ba54ba58bdaa85c2519c84adaabecd8ee48bff1",
        "modern/accountauth/records_test.go": "264aa1b384ce201dc5d69f92488d2ec0807de59ef048b46118f0f96608dd8fe9",
        "modern/accountauth/state_test.go": "949066a95bd9a8cda7b0bcedbbd3bea25f36185f9b16db6f48849638dbeef9ed",
        "modern/accountauth/fuzz_test.go": "08245bcfdc8d2620f0499c69c2eacdd2033c47beec68959b080bd2d9f960b833",
        "modern/go.mod": "1150b94372852355beaffa7104430a21e8f8aa6ec4877bad18d0bcdb71453783",
        "modern/go.sum": "4c91209822dccd4a60955ddd6b8b94a327e88b55721577494c953a705395b83a",
        "modern/network/open.go": "96bf07274832c57ef67ace9a6e0a5dc06651bc7a5b27eef7cff7f480610d4f3b",
        "modern/accountstore/store_test.go": "6c7c3b4c0849670fc4b1527091b0fb40e26854db10013813c885f5cff5940e32",
        "modern/accountstore/persistence_test.go": "25e8308df9ff6a6aa3d1e43747a6d9a09efd66c12d36a6c3e19e1ef3db9e4c3a",
        "modern/accountstore/fuzz_test.go": "ecff630ee50e23b4651d34aab7190f4872b2f8ad2e0d91b718e31d3518bcc9af",
        "modern/accountstore/types.go": "d10523e39dc4b0e40b80be7105094c1df0d514dd3e156509e47de9a753f7b24f",
        "modern/accountstore/store.go": "7bd1a76b1055bcaa1723f751c702aa70fd4e6d40a6947267c3c1a6c1ab9b6d81",
        "modern/accountstore/codec.go": "fe864efcf92f2f2168da37fa7ea3b482b35f0463e438023dad27a10470e0deab"
      }
    },
    {
      "name": "green",
      "argv": [
        "<home>/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.linux-amd64/bin/go",
        "test",
        "./accountstore",
        "-count=1",
        "-timeout=3m"
      ],
      "cwd": "<repo>/modern",
      "started_utc": "2026-09-17T19:21:11.915764+00:00",
      "returncode": 0,
      "finished_utc": "2026-09-17T19:21:25.556355+00:00",
      "stdout_sha256": "6304010553362b507f1e7623b0fef14b28bb4a76a03f6f38efe93d9ccda18a24",
      "stderr_sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
      "after": {
        "modern/accountauth/types.go": "ac264e0d967543f915d8a305eda7a78e470dcb6146b2048305fc10070fe98dfb",
        "modern/accountauth/records.go": "cafbb6b14c7fab54df0d14e9cf5822c85f8d0d9e5a98c2aa68c56ea705aa0640",
        "modern/accountauth/state.go": "b1807c69ffaf155dd2a4fd8f1ba54ba58bdaa85c2519c84adaabecd8ee48bff1",
        "modern/accountauth/records_test.go": "264aa1b384ce201dc5d69f92488d2ec0807de59ef048b46118f0f96608dd8fe9",
        "modern/accountauth/state_test.go": "949066a95bd9a8cda7b0bcedbbd3bea25f36185f9b16db6f48849638dbeef9ed",
        "modern/accountauth/fuzz_test.go": "08245bcfdc8d2620f0499c69c2eacdd2033c47beec68959b080bd2d9f960b833",
        "modern/go.mod": "1150b94372852355beaffa7104430a21e8f8aa6ec4877bad18d0bcdb71453783",
        "modern/go.sum": "4c91209822dccd4a60955ddd6b8b94a327e88b55721577494c953a705395b83a",
        "modern/network/open.go": "96bf07274832c57ef67ace9a6e0a5dc06651bc7a5b27eef7cff7f480610d4f3b",
        "modern/accountstore/store_test.go": "6c7c3b4c0849670fc4b1527091b0fb40e26854db10013813c885f5cff5940e32",
        "modern/accountstore/persistence_test.go": "25e8308df9ff6a6aa3d1e43747a6d9a09efd66c12d36a6c3e19e1ef3db9e4c3a",
        "modern/accountstore/fuzz_test.go": "ecff630ee50e23b4651d34aab7190f4872b2f8ad2e0d91b718e31d3518bcc9af",
        "modern/accountstore/types.go": "d10523e39dc4b0e40b80be7105094c1df0d514dd3e156509e47de9a753f7b24f",
        "modern/accountstore/store.go": "7bd1a76b1055bcaa1723f751c702aa70fd4e6d40a6947267c3c1a6c1ab9b6d81",
        "modern/accountstore/codec.go": "fe864efcf92f2f2168da37fa7ea3b482b35f0463e438023dad27a10470e0deab"
      }
    },
    {
      "name": "race",
      "argv": [
        "<home>/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.linux-amd64/bin/go",
        "test",
        "-race",
        "./accountauth",
        "./accountstore",
        "-count=1",
        "-timeout=3m"
      ],
      "cwd": "<repo>/modern",
      "started_utc": "2026-09-17T19:21:25.557092+00:00",
      "returncode": 0,
      "finished_utc": "2026-09-17T19:22:15.630318+00:00",
      "stdout_sha256": "aaa1842cd7d06b02b3d32e913590d007fb61315d3607ba1679869648f08b0529",
      "stderr_sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
      "after": {
        "modern/accountauth/types.go": "ac264e0d967543f915d8a305eda7a78e470dcb6146b2048305fc10070fe98dfb",
        "modern/accountauth/records.go": "cafbb6b14c7fab54df0d14e9cf5822c85f8d0d9e5a98c2aa68c56ea705aa0640",
        "modern/accountauth/state.go": "b1807c69ffaf155dd2a4fd8f1ba54ba58bdaa85c2519c84adaabecd8ee48bff1",
        "modern/accountauth/records_test.go": "264aa1b384ce201dc5d69f92488d2ec0807de59ef048b46118f0f96608dd8fe9",
        "modern/accountauth/state_test.go": "949066a95bd9a8cda7b0bcedbbd3bea25f36185f9b16db6f48849638dbeef9ed",
        "modern/accountauth/fuzz_test.go": "08245bcfdc8d2620f0499c69c2eacdd2033c47beec68959b080bd2d9f960b833",
        "modern/go.mod": "1150b94372852355beaffa7104430a21e8f8aa6ec4877bad18d0bcdb71453783",
        "modern/go.sum": "4c91209822dccd4a60955ddd6b8b94a327e88b55721577494c953a705395b83a",
        "modern/network/open.go": "96bf07274832c57ef67ace9a6e0a5dc06651bc7a5b27eef7cff7f480610d4f3b",
        "modern/accountstore/store_test.go": "6c7c3b4c0849670fc4b1527091b0fb40e26854db10013813c885f5cff5940e32",
        "modern/accountstore/persistence_test.go": "25e8308df9ff6a6aa3d1e43747a6d9a09efd66c12d36a6c3e19e1ef3db9e4c3a",
        "modern/accountstore/fuzz_test.go": "ecff630ee50e23b4651d34aab7190f4872b2f8ad2e0d91b718e31d3518bcc9af",
        "modern/accountstore/types.go": "d10523e39dc4b0e40b80be7105094c1df0d514dd3e156509e47de9a753f7b24f",
        "modern/accountstore/store.go": "7bd1a76b1055bcaa1723f751c702aa70fd4e6d40a6947267c3c1a6c1ab9b6d81",
        "modern/accountstore/codec.go": "fe864efcf92f2f2168da37fa7ea3b482b35f0463e438023dad27a10470e0deab"
      }
    },
    {
      "name": "vet",
      "argv": [
        "<home>/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.linux-amd64/bin/go",
        "vet",
        "./accountstore"
      ],
      "cwd": "<repo>/modern",
      "started_utc": "2026-09-17T19:22:15.630969+00:00",
      "returncode": 0,
      "finished_utc": "2026-09-17T19:22:17.097364+00:00",
      "stdout_sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
      "stderr_sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
      "after": {
        "modern/accountauth/types.go": "ac264e0d967543f915d8a305eda7a78e470dcb6146b2048305fc10070fe98dfb",
        "modern/accountauth/records.go": "cafbb6b14c7fab54df0d14e9cf5822c85f8d0d9e5a98c2aa68c56ea705aa0640",
        "modern/accountauth/state.go": "b1807c69ffaf155dd2a4fd8f1ba54ba58bdaa85c2519c84adaabecd8ee48bff1",
        "modern/accountauth/records_test.go": "264aa1b384ce201dc5d69f92488d2ec0807de59ef048b46118f0f96608dd8fe9",
        "modern/accountauth/state_test.go": "949066a95bd9a8cda7b0bcedbbd3bea25f36185f9b16db6f48849638dbeef9ed",
        "modern/accountauth/fuzz_test.go": "08245bcfdc8d2620f0499c69c2eacdd2033c47beec68959b080bd2d9f960b833",
        "modern/go.mod": "1150b94372852355beaffa7104430a21e8f8aa6ec4877bad18d0bcdb71453783",
        "modern/go.sum": "4c91209822dccd4a60955ddd6b8b94a327e88b55721577494c953a705395b83a",
        "modern/network/open.go": "96bf07274832c57ef67ace9a6e0a5dc06651bc7a5b27eef7cff7f480610d4f3b",
        "modern/accountstore/store_test.go": "6c7c3b4c0849670fc4b1527091b0fb40e26854db10013813c885f5cff5940e32",
        "modern/accountstore/persistence_test.go": "25e8308df9ff6a6aa3d1e43747a6d9a09efd66c12d36a6c3e19e1ef3db9e4c3a",
        "modern/accountstore/fuzz_test.go": "ecff630ee50e23b4651d34aab7190f4872b2f8ad2e0d91b718e31d3518bcc9af",
        "modern/accountstore/types.go": "d10523e39dc4b0e40b80be7105094c1df0d514dd3e156509e47de9a753f7b24f",
        "modern/accountstore/store.go": "7bd1a76b1055bcaa1723f751c702aa70fd4e6d40a6947267c3c1a6c1ab9b6d81",
        "modern/accountstore/codec.go": "fe864efcf92f2f2168da37fa7ea3b482b35f0463e438023dad27a10470e0deab"
      }
    },
    {
      "name": "fuzz",
      "argv": [
        "<home>/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.linux-amd64/bin/go",
        "test",
        "./accountstore",
        "-run",
        "^$",
        "-fuzz",
        "^FuzzOpenSnapshot$",
        "-fuzztime=30s",
        "-parallel=2",
        "-timeout=3m"
      ],
      "cwd": "<repo>/modern",
      "started_utc": "2026-09-17T19:22:17.098165+00:00",
      "returncode": 0,
      "finished_utc": "2026-09-17T19:22:59.945167+00:00",
      "stdout_sha256": "eb107071ecfc65407cc340e77c0e2967149b2f9459af95192d793a7ce6ee6056",
      "stderr_sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
      "after": {
        "modern/accountauth/types.go": "ac264e0d967543f915d8a305eda7a78e470dcb6146b2048305fc10070fe98dfb",
        "modern/accountauth/records.go": "cafbb6b14c7fab54df0d14e9cf5822c85f8d0d9e5a98c2aa68c56ea705aa0640",
        "modern/accountauth/state.go": "b1807c69ffaf155dd2a4fd8f1ba54ba58bdaa85c2519c84adaabecd8ee48bff1",
        "modern/accountauth/records_test.go": "264aa1b384ce201dc5d69f92488d2ec0807de59ef048b46118f0f96608dd8fe9",
        "modern/accountauth/state_test.go": "949066a95bd9a8cda7b0bcedbbd3bea25f36185f9b16db6f48849638dbeef9ed",
        "modern/accountauth/fuzz_test.go": "08245bcfdc8d2620f0499c69c2eacdd2033c47beec68959b080bd2d9f960b833",
        "modern/go.mod": "1150b94372852355beaffa7104430a21e8f8aa6ec4877bad18d0bcdb71453783",
        "modern/go.sum": "4c91209822dccd4a60955ddd6b8b94a327e88b55721577494c953a705395b83a",
        "modern/network/open.go": "96bf07274832c57ef67ace9a6e0a5dc06651bc7a5b27eef7cff7f480610d4f3b",
        "modern/accountstore/store_test.go": "6c7c3b4c0849670fc4b1527091b0fb40e26854db10013813c885f5cff5940e32",
        "modern/accountstore/persistence_test.go": "25e8308df9ff6a6aa3d1e43747a6d9a09efd66c12d36a6c3e19e1ef3db9e4c3a",
        "modern/accountstore/fuzz_test.go": "ecff630ee50e23b4651d34aab7190f4872b2f8ad2e0d91b718e31d3518bcc9af",
        "modern/accountstore/types.go": "d10523e39dc4b0e40b80be7105094c1df0d514dd3e156509e47de9a753f7b24f",
        "modern/accountstore/store.go": "7bd1a76b1055bcaa1723f751c702aa70fd4e6d40a6947267c3c1a6c1ab9b6d81",
        "modern/accountstore/codec.go": "fe864efcf92f2f2168da37fa7ea3b482b35f0463e438023dad27a10470e0deab"
      }
    },
    {
      "name": "fault",
      "argv": [
        "<home>/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.linux-amd64/bin/go",
        "test",
        "-overlay=<repo>/modern/dist/acc002/green-20260917T192111797366Z/overlay.json",
        "./accountstore",
        "-run",
        "^TestRevocationSurvivesReopen$",
        "-count=1",
        "-timeout=3m"
      ],
      "cwd": "<repo>/modern",
      "started_utc": "2026-09-17T19:22:59.946341+00:00",
      "returncode": 1,
      "finished_utc": "2026-09-17T19:23:00.511221+00:00",
      "stdout_sha256": "d9804cdc71e68ffe787835aa669757628a87cadb528d839df1bcddfcf3852278",
      "stderr_sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
      "after": {
        "modern/accountauth/types.go": "ac264e0d967543f915d8a305eda7a78e470dcb6146b2048305fc10070fe98dfb",
        "modern/accountauth/records.go": "cafbb6b14c7fab54df0d14e9cf5822c85f8d0d9e5a98c2aa68c56ea705aa0640",
        "modern/accountauth/state.go": "b1807c69ffaf155dd2a4fd8f1ba54ba58bdaa85c2519c84adaabecd8ee48bff1",
        "modern/accountauth/records_test.go": "264aa1b384ce201dc5d69f92488d2ec0807de59ef048b46118f0f96608dd8fe9",
        "modern/accountauth/state_test.go": "949066a95bd9a8cda7b0bcedbbd3bea25f36185f9b16db6f48849638dbeef9ed",
        "modern/accountauth/fuzz_test.go": "08245bcfdc8d2620f0499c69c2eacdd2033c47beec68959b080bd2d9f960b833",
        "modern/go.mod": "1150b94372852355beaffa7104430a21e8f8aa6ec4877bad18d0bcdb71453783",
        "modern/go.sum": "4c91209822dccd4a60955ddd6b8b94a327e88b55721577494c953a705395b83a",
        "modern/network/open.go": "96bf07274832c57ef67ace9a6e0a5dc06651bc7a5b27eef7cff7f480610d4f3b",
        "modern/accountstore/store_test.go": "6c7c3b4c0849670fc4b1527091b0fb40e26854db10013813c885f5cff5940e32",
        "modern/accountstore/persistence_test.go": "25e8308df9ff6a6aa3d1e43747a6d9a09efd66c12d36a6c3e19e1ef3db9e4c3a",
        "modern/accountstore/fuzz_test.go": "ecff630ee50e23b4651d34aab7190f4872b2f8ad2e0d91b718e31d3518bcc9af",
        "modern/accountstore/types.go": "d10523e39dc4b0e40b80be7105094c1df0d514dd3e156509e47de9a753f7b24f",
        "modern/accountstore/store.go": "7bd1a76b1055bcaa1723f751c702aa70fd4e6d40a6947267c3c1a6c1ab9b6d81",
        "modern/accountstore/codec.go": "fe864efcf92f2f2168da37fa7ea3b482b35f0463e438023dad27a10470e0deab"
      }
    },
    {
      "name": "restored",
      "argv": [
        "<home>/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.linux-amd64/bin/go",
        "test",
        "./accountstore",
        "-run",
        "^TestRevocationSurvivesReopen$",
        "-count=1",
        "-timeout=3m"
      ],
      "cwd": "<repo>/modern",
      "started_utc": "2026-09-17T19:23:00.512093+00:00",
      "returncode": 0,
      "finished_utc": "2026-09-17T19:23:00.826480+00:00",
      "stdout_sha256": "cebb2386288bfc7d1693cb1baaf4b97bb01ef22de2e8ccf826b3dbfa12999d37",
      "stderr_sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
      "after": {
        "modern/accountauth/types.go": "ac264e0d967543f915d8a305eda7a78e470dcb6146b2048305fc10070fe98dfb",
        "modern/accountauth/records.go": "cafbb6b14c7fab54df0d14e9cf5822c85f8d0d9e5a98c2aa68c56ea705aa0640",
        "modern/accountauth/state.go": "b1807c69ffaf155dd2a4fd8f1ba54ba58bdaa85c2519c84adaabecd8ee48bff1",
        "modern/accountauth/records_test.go": "264aa1b384ce201dc5d69f92488d2ec0807de59ef048b46118f0f96608dd8fe9",
        "modern/accountauth/state_test.go": "949066a95bd9a8cda7b0bcedbbd3bea25f36185f9b16db6f48849638dbeef9ed",
        "modern/accountauth/fuzz_test.go": "08245bcfdc8d2620f0499c69c2eacdd2033c47beec68959b080bd2d9f960b833",
        "modern/go.mod": "1150b94372852355beaffa7104430a21e8f8aa6ec4877bad18d0bcdb71453783",
        "modern/go.sum": "4c91209822dccd4a60955ddd6b8b94a327e88b55721577494c953a705395b83a",
        "modern/network/open.go": "96bf07274832c57ef67ace9a6e0a5dc06651bc7a5b27eef7cff7f480610d4f3b",
        "modern/accountstore/store_test.go": "6c7c3b4c0849670fc4b1527091b0fb40e26854db10013813c885f5cff5940e32",
        "modern/accountstore/persistence_test.go": "25e8308df9ff6a6aa3d1e43747a6d9a09efd66c12d36a6c3e19e1ef3db9e4c3a",
        "modern/accountstore/fuzz_test.go": "ecff630ee50e23b4651d34aab7190f4872b2f8ad2e0d91b718e31d3518bcc9af",
        "modern/accountstore/types.go": "d10523e39dc4b0e40b80be7105094c1df0d514dd3e156509e47de9a753f7b24f",
        "modern/accountstore/store.go": "7bd1a76b1055bcaa1723f751c702aa70fd4e6d40a6947267c3c1a6c1ab9b6d81",
        "modern/accountstore/codec.go": "fe864efcf92f2f2168da37fa7ea3b482b35f0463e438023dad27a10470e0deab"
      }
    },
    {
      "name": "gosec",
      "argv": [
        "<home>/go/bin/gosec",
        "-tests",
        "./accountstore/..."
      ],
      "cwd": "<repo>/modern",
      "started_utc": "2026-09-17T19:23:00.827310+00:00",
      "returncode": 1,
      "finished_utc": "2026-09-17T19:23:01.342011+00:00",
      "stdout_sha256": "b246c5d1940522a464d993f21a95b11d2abfef04672ae9d917645c0460c1bc3f",
      "stderr_sha256": "abb041bfeb35c7b202a9d860b7f26d51dfdb62f89a1dacbfabfa8b4edc706261",
      "after": {
        "modern/accountauth/types.go": "ac264e0d967543f915d8a305eda7a78e470dcb6146b2048305fc10070fe98dfb",
        "modern/accountauth/records.go": "cafbb6b14c7fab54df0d14e9cf5822c85f8d0d9e5a98c2aa68c56ea705aa0640",
        "modern/accountauth/state.go": "b1807c69ffaf155dd2a4fd8f1ba54ba58bdaa85c2519c84adaabecd8ee48bff1",
        "modern/accountauth/records_test.go": "264aa1b384ce201dc5d69f92488d2ec0807de59ef048b46118f0f96608dd8fe9",
        "modern/accountauth/state_test.go": "949066a95bd9a8cda7b0bcedbbd3bea25f36185f9b16db6f48849638dbeef9ed",
        "modern/accountauth/fuzz_test.go": "08245bcfdc8d2620f0499c69c2eacdd2033c47beec68959b080bd2d9f960b833",
        "modern/go.mod": "1150b94372852355beaffa7104430a21e8f8aa6ec4877bad18d0bcdb71453783",
        "modern/go.sum": "4c91209822dccd4a60955ddd6b8b94a327e88b55721577494c953a705395b83a",
        "modern/network/open.go": "96bf07274832c57ef67ace9a6e0a5dc06651bc7a5b27eef7cff7f480610d4f3b",
        "modern/accountstore/store_test.go": "6c7c3b4c0849670fc4b1527091b0fb40e26854db10013813c885f5cff5940e32",
        "modern/accountstore/persistence_test.go": "25e8308df9ff6a6aa3d1e43747a6d9a09efd66c12d36a6c3e19e1ef3db9e4c3a",
        "modern/accountstore/fuzz_test.go": "ecff630ee50e23b4651d34aab7190f4872b2f8ad2e0d91b718e31d3518bcc9af",
        "modern/accountstore/types.go": "d10523e39dc4b0e40b80be7105094c1df0d514dd3e156509e47de9a753f7b24f",
        "modern/accountstore/store.go": "7bd1a76b1055bcaa1723f751c702aa70fd4e6d40a6947267c3c1a6c1ab9b6d81",
        "modern/accountstore/codec.go": "fe864efcf92f2f2168da37fa7ea3b482b35f0463e438023dad27a10470e0deab"
      }
    },
    {
      "name": "govulncheck-policy",
      "argv": [
        "/usr/bin/python3",
        "-c",
        "import datetime, json, pathlib, runpy, subprocess, sys\nrun = pathlib.Path(sys.argv[2])\nns = runpy.run_path(sys.argv[1], run_name='acc002_policy')\ndef execute(argv, **kwargs):\n    utc = lambda: datetime.datetime.now(datetime.timezone.utc).isoformat()\n    entry = {'argv': argv, 'cwd': kwargs.get('cwd'), 'started_utc': utc()}\n    result = subprocess.run(argv, **kwargs)\n    (run / 'govulncheck.sarif').write_text(result.stdout or '')\n    (run / 'govulncheck.stderr').write_text(result.stderr or '')\n    entry.update(returncode=result.returncode, finished_utc=utc())\n    (run / 'govulncheck-command.json').write_text(json.dumps(entry, indent=2))\n    return result\nraise SystemExit(ns['main'](['source'], execute=execute))\n",
        "<repo>/scripts/govulncheck_policy.py",
        "<repo>/modern/dist/acc002/green-20260917T192111797366Z"
      ],
      "cwd": "<repo>",
      "started_utc": "2026-09-17T19:23:01.342917+00:00",
      "returncode": 1,
      "finished_utc": "2026-09-17T19:23:07.620214+00:00",
      "stdout_sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
      "stderr_sha256": "007d48a08fdc6d7f4f005fbb1168e38312c433e71495b6eb6b63e22c642807ca",
      "after": {
        "modern/accountauth/types.go": "ac264e0d967543f915d8a305eda7a78e470dcb6146b2048305fc10070fe98dfb",
        "modern/accountauth/records.go": "cafbb6b14c7fab54df0d14e9cf5822c85f8d0d9e5a98c2aa68c56ea705aa0640",
        "modern/accountauth/state.go": "b1807c69ffaf155dd2a4fd8f1ba54ba58bdaa85c2519c84adaabecd8ee48bff1",
        "modern/accountauth/records_test.go": "264aa1b384ce201dc5d69f92488d2ec0807de59ef048b46118f0f96608dd8fe9",
        "modern/accountauth/state_test.go": "949066a95bd9a8cda7b0bcedbbd3bea25f36185f9b16db6f48849638dbeef9ed",
        "modern/accountauth/fuzz_test.go": "08245bcfdc8d2620f0499c69c2eacdd2033c47beec68959b080bd2d9f960b833",
        "modern/go.mod": "1150b94372852355beaffa7104430a21e8f8aa6ec4877bad18d0bcdb71453783",
        "modern/go.sum": "4c91209822dccd4a60955ddd6b8b94a327e88b55721577494c953a705395b83a",
        "modern/network/open.go": "96bf07274832c57ef67ace9a6e0a5dc06651bc7a5b27eef7cff7f480610d4f3b",
        "modern/accountstore/store_test.go": "6c7c3b4c0849670fc4b1527091b0fb40e26854db10013813c885f5cff5940e32",
        "modern/accountstore/persistence_test.go": "25e8308df9ff6a6aa3d1e43747a6d9a09efd66c12d36a6c3e19e1ef3db9e4c3a",
        "modern/accountstore/fuzz_test.go": "ecff630ee50e23b4651d34aab7190f4872b2f8ad2e0d91b718e31d3518bcc9af",
        "modern/accountstore/types.go": "d10523e39dc4b0e40b80be7105094c1df0d514dd3e156509e47de9a753f7b24f",
        "modern/accountstore/store.go": "7bd1a76b1055bcaa1723f751c702aa70fd4e6d40a6947267c3c1a6c1ab9b6d81",
        "modern/accountstore/codec.go": "fe864efcf92f2f2168da37fa7ea3b482b35f0463e438023dad27a10470e0deab"
      }
    }
  ],
  "fault_sha256": "f9f5b359b8f7414d28cc9bfcf45b35cd3236735a03e4767615fc8468480e2849",
  "overlay_sha256": "bb295eb07821b31c959433eb8a93a584fee604ed6a5b6d6c6dfcb6f2a46370de",
  "fault_detected": true,
  "after": {
    "modern/accountauth/types.go": "ac264e0d967543f915d8a305eda7a78e470dcb6146b2048305fc10070fe98dfb",
    "modern/accountauth/records.go": "cafbb6b14c7fab54df0d14e9cf5822c85f8d0d9e5a98c2aa68c56ea705aa0640",
    "modern/accountauth/state.go": "b1807c69ffaf155dd2a4fd8f1ba54ba58bdaa85c2519c84adaabecd8ee48bff1",
    "modern/accountauth/records_test.go": "264aa1b384ce201dc5d69f92488d2ec0807de59ef048b46118f0f96608dd8fe9",
    "modern/accountauth/state_test.go": "949066a95bd9a8cda7b0bcedbbd3bea25f36185f9b16db6f48849638dbeef9ed",
    "modern/accountauth/fuzz_test.go": "08245bcfdc8d2620f0499c69c2eacdd2033c47beec68959b080bd2d9f960b833",
    "modern/go.mod": "1150b94372852355beaffa7104430a21e8f8aa6ec4877bad18d0bcdb71453783",
    "modern/go.sum": "4c91209822dccd4a60955ddd6b8b94a327e88b55721577494c953a705395b83a",
    "modern/network/open.go": "96bf07274832c57ef67ace9a6e0a5dc06651bc7a5b27eef7cff7f480610d4f3b",
    "modern/accountstore/store_test.go": "6c7c3b4c0849670fc4b1527091b0fb40e26854db10013813c885f5cff5940e32",
    "modern/accountstore/persistence_test.go": "25e8308df9ff6a6aa3d1e43747a6d9a09efd66c12d36a6c3e19e1ef3db9e4c3a",
    "modern/accountstore/fuzz_test.go": "ecff630ee50e23b4651d34aab7190f4872b2f8ad2e0d91b718e31d3518bcc9af",
    "modern/accountstore/types.go": "d10523e39dc4b0e40b80be7105094c1df0d514dd3e156509e47de9a753f7b24f",
    "modern/accountstore/store.go": "7bd1a76b1055bcaa1723f751c702aa70fd4e6d40a6947267c3c1a6c1ab9b6d81",
    "modern/accountstore/codec.go": "fe864efcf92f2f2168da37fa7ea3b482b35f0463e438023dad27a10470e0deab"
  },
  "source_unchanged": true,
  "finished_utc": "2026-09-17T19:23:07.621580+00:00",
  "artifacts": {
    "store-fault.go": "f9f5b359b8f7414d28cc9bfcf45b35cd3236735a03e4767615fc8468480e2849",
    "govulncheck-policy.stdout": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    "race.stderr": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    "go-version.stdout": "76227025cc0bc2be7067aa45d11e09cacfd49c58f498f4c2e4f6a9872a607bf9",
    "vet.stdout": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    "govulncheck.stderr": "870036455d222804f8d1b74aeb02df5acfd6d8d02c1612de4f6ac0cbc94e87a7",
    "scanner-identities.stdout": "d9a59be6752c7fa194dc5e5b5040cf7e3e2686589f11a28c189485384dafe3e1",
    "restored.stdout": "cebb2386288bfc7d1693cb1baaf4b97bb01ef22de2e8ccf826b3dbfa12999d37",
    "restored.stderr": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    "green.stdout": "6304010553362b507f1e7623b0fef14b28bb4a76a03f6f38efe93d9ccda18a24",
    "govulncheck-command.json": "fa3281cfe9a1631d6f752b5504081b63ff6f7213b536a610768c3ebf219fa240",
    "vet.stderr": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    "scanner-identities.stderr": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    "gosec.stdout": "b246c5d1940522a464d993f21a95b11d2abfef04672ae9d917645c0460c1bc3f",
    "fuzz.stdout": "eb107071ecfc65407cc340e77c0e2967149b2f9459af95192d793a7ce6ee6056",
    "overlay.json": "bb295eb07821b31c959433eb8a93a584fee604ed6a5b6d6c6dfcb6f2a46370de",
    "green.stderr": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    "race.stdout": "aaa1842cd7d06b02b3d32e913590d007fb61315d3607ba1679869648f08b0529",
    "govulncheck.sarif": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    "go-version.stderr": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    "fault.stdout": "d9804cdc71e68ffe787835aa669757628a87cadb528d839df1bcddfcf3852278",
    "govulncheck-policy.stderr": "007d48a08fdc6d7f4f005fbb1168e38312c433e71495b6eb6b63e22c642807ca",
    "fuzz.stderr": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    "gosec.stderr": "abb041bfeb35c7b202a9d860b7f26d51dfdb62f89a1dacbfabfa8b4edc706261",
    "fault.stderr": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
  }
}
```

### go-version

stdout:

```text
go version go1.27.0 linux/amd64
```

stderr:

```text

```

### scanner-identities

stdout:

```text
<home>/go/bin/gosec: go1.27.0
	path	github.com/securego/gosec/v2/cmd/gosec
	mod	github.com/securego/gosec/v2	v2.29.0	h1:pF2HSLcnY5voqpxQumEe0O5YAAbvF4Syu4qeIewpLyc=
	dep	cloud.google.com/go	v0.123.0	h1:2NAUJwPR47q+E35uaJeYoNhuNEM9kM8SjgRgdeOJUSE=
	dep	cloud.google.com/go/auth	v0.23.2	h1:[REDACTED-GOSEC-DEPS-HASH]=
	dep	cloud.google.com/go/compute/metadata	v0.9.0	h1:pDUj4QMoPejqq20dK0Pg2N4yG9zIkYGdBtwLoEkH9Zs=
	dep	github.com/anthropics/anthropic-sdk-go	v1.66.0	h1:/CKwgscn0Pe1q4U8aFInSOt/v06JeMc9Aq4vIlctCFw=
	dep	github.com/bahlo/generic-list-go	v0.2.0	h1:5sz/EEAK+ls5wF+NeqDpk5+iNdMDXrh3z3nPnH1Wvgk=
	dep	github.com/buger/jsonparser	v1.6.1	h1:I0phFv0PlbLHnM7TZAVjZ2MJ2/eWRTDyuO7GLR98IEs=
	dep	github.com/ccojocar/zxcvbn-go	v1.0.4	h1:FWnCIRMXPj43ukfX000kvBZvV6raSxakYr1nzyNrUcc=
	dep	github.com/cespare/xxhash/v2	v2.3.0	h1:UL815xU9SqsFlibzuggzjXhog7bL6oX9BbNZnL2UFvs=
	dep	github.com/felixge/httpsnoop	v1.1.0	h1:3YtUj32ZZkqZtt3sZZsClsymw/QDuVfpNhoA31zeORc=
	dep	github.com/go-logr/logr	v1.4.4	h1:tG4xh9yMsRCAiodLVTxyrkzSZ9+o0L1Kg/+cPVcbP/8=
	dep	github.com/go-logr/stdr	v1.2.2	h1:hSWxHoqTgW2S2qGc0LTAI563KZ5YKYRhT3MFKZMbjag=
	dep	github.com/google/go-cmp	v0.7.0	h1:wk8382ETsv4JYUZwIsn6YpYiWiBsYLSJiTsyBybVuN8=
	dep	github.com/google/s2a-go	v0.1.9	h1:LGD7gtMgezd8a/Xak7mEWL0PjoTQFvpRudN895yqKW0=
	dep	github.com/google/uuid	v1.6.0	h1:NIvaJDMOsjHA8n1jAhLSgzrAzy1Hgr+hNrb57e+94F0=
	dep	github.com/googleapis/enterprise-certificate-proxy	v0.3.20	h1:t/xL64VUoN69MuMRQuJETqYGOw4Z9mSRJK9epIEtwFk=
	dep	github.com/googleapis/gax-go/v2	v2.24.0	h1:myMaPYyF9MecEmvQqMqomIwn9t/4KCZN9qnwsS76wlg=
	dep	github.com/gookit/color	v1.6.1	h1:KoTnDxJPRgrL0SoX0f8rCFg2zI0t4E3GZZBMo2nN8LU=
	dep	github.com/gorilla/websocket	v1.5.3	h1:saDtZ6Pbx/0u+bgYQ3q96pZgCzfhKXGPqt7kZ72aNNg=
	dep	github.com/invopop/jsonschema	v0.14.0	h1:MHQqLhvpNUZfw+hM3AZDYK7jxO8FZoQeQM77g8iyZjg=
	dep	github.com/openai/openai-go/v3	v3.52.0	h1:VDSjIvI5Sr2/AzGJI6219sM2Il+zBWuopvluMy6KdjE=
	dep	github.com/pb33f/ordered-map/v2	v2.3.1	h1:5319HDO0aw4DA4gzi+zv4FXU9UlSs3xGZ40wcP1nBjY=
	dep	github.com/standard-webhooks/standard-webhooks/libraries	v0.0.1	h1:uOfcYT+3QungH6tIGSVCR/Y3KJmgJiHcojJbMTPDZAI=
	dep	github.com/tidwall/gjson	v1.19.0	h1:xwxm7n691Uf3u5OFjzngavjGTh55KX5q/9w9xHW88JU=
	dep	github.com/tidwall/match	v1.2.0	h1:0pt8FlkOwjN2fPt4bIl4BoNxb98gGHN2ObFEDkrfZnM=
	dep	github.com/tidwall/pretty	v1.2.1	h1:qjsOFOWWQl+N3RsoF5/ssm1pHmJJwhjlSbZ51I6wMl4=
	dep	github.com/tidwall/sjson	v1.2.5	h1:kLy8mja+1c9jlljvWTlSazM7cKDRfJuR/bOJhcY5NcY=
	dep	github.com/xo/terminfo	v1.0.0	h1:2ZpYzqWzyyytjk3TP6aJVDhkMAkc99/1xKQdA3TDTBY=
	dep	go.opentelemetry.io/auto/sdk	v1.2.1	h1:jXsnJ4Lmnqd11kwkBV2LgLoFMZKizbCi5fNZ/ipaZ64=
	dep	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp	v0.70.0	h1:LMuyCAyfalSjDyjdC65nK6N0zoTT63+E/u95X0JovZI=
	dep	go.opentelemetry.io/otel	v1.45.0	h1:pdrWmLHofpubmArBv1LgFSv1Z0Ie/ppdZzu+kUN5EeU=
	dep	go.opentelemetry.io/otel/metric	v1.45.0	h1:7Eg1uH7CJ5cXv9is6tnBe1FI6rj1nwUdbFypRm3br/M=
	dep	go.opentelemetry.io/otel/trace	v1.45.0	h1:l/mP6Uv7oNO7/TblbhpbgMidxhq1uO/rPsikOyVhxag=
	dep	go.yaml.in/yaml/v3	v3.0.5	h1:N6y/pJk8buWs9NY5ERU2HSMfm+IuD/OtfdAnq6kESPw=
	dep	go.yaml.in/yaml/v4	v4.0.0-rc.6	h1:1h7H1ohdUh93/FyE4YaDa1Zh64K6VVbjF4K6WUxMtH4=
	dep	golang.org/x/crypto	v0.55.0	h1:+KWHjbgOaAQ66dh/YlkZKHlz9ZUlq61AFirAR9ntP8M=
	dep	golang.org/x/mod	v0.40.0	h1:hUv+3cXcdRHz08UmSiOob7sadHig73uo5bkXxQ/tvUs=
	dep	golang.org/x/net	v0.58.0	h1:ynWG7rqYi4ccpTEuPZ2QGWHktVEM9DMCj9yzDE0Q7To=
	dep	golang.org/x/sync	v0.22.0	h1:SZjpbeLmrCk4xhRSZFNZW5gFUeCeFgjekvI/+gfScek=
	dep	golang.org/x/sys	v0.47.0	h1:o7XGOvZQCADBQQ4Y7VNq2dRWQR7JmOUW8Kxx4ZsNgWs=
	dep	golang.org/x/text	v0.41.0	h1:vz/seA0lnX87Othu2f/0L24RcgrXD9/YFTSuGjj3rH8=
	dep	golang.org/x/tools	v0.49.0	h1:3NI7VXzL9+1WZD52Dx2ttoPwD5DWrFGpl9mFZDlmisI=
	dep	google.golang.org/api	v0.293.0	h1:p9XIWOf63U4OgYx120ZwVU8+vl4XTPmWfgVPnmOAS9w=
	dep	google.golang.org/genai	v1.69.0	h1:quP3Rbiz0Mn+zPfXsWHQQwOx8IfO2MnQehZUbrJ/jPo=
	dep	google.golang.org/genproto/googleapis/rpc	v0.0.0-20260819154853-08b0e4226688	h1:cYNAzI2sUwhmCcoj9TxvihSrqsxt6uIkj3rDRhSDmW4=
	dep	google.golang.org/grpc	v1.83.1	h1:HIO0+BEtBP6soyqvqC8sNUjZ7bTs+0hFQuFF+RAy++Y=
	dep	google.golang.org/protobuf	v1.36.12	h1:pJOKDDOyeXErUroCihFAd5LQuwXBSpVnKGrj5o/fwxc=
	build	-buildmode=exe
	build	-compiler=gc
	build	DefaultGODEBUG=cryptocustomrand=1,tlssecpmlkem=0,tracebacklabels=0,urlstrictcolons=0,x509sslcertoverrideplatform=0
	build	CGO_ENABLED=1
	build	CGO_CFLAGS=
	build	CGO_CPPFLAGS=
	build	CGO_CXXFLAGS=
	build	CGO_LDFLAGS=
	build	GOARCH=amd64
	build	GOOS=linux
	build	GOAMD64=v1
<home>/go/bin/govulncheck: go1.27.0
	path	golang.org/x/vuln/cmd/govulncheck
	mod	golang.org/x/vuln	v1.7.0	h1:4MQBuhmXbz2uepNJrf3v+aaZLGDqw1JluwYboegA1qg=
	dep	golang.org/x/mod	v0.39.0	h1:UF5zwQdCRRUpHfyPwr7d4UrGiVeldIsogtzWVnczL74=
	dep	golang.org/x/sync	v0.22.0	h1:SZjpbeLmrCk4xhRSZFNZW5gFUeCeFgjekvI/+gfScek=
	dep	golang.org/x/telemetry	v0.0.0-20260811182544-a038080d80e5	h1:ZUSxONxc981v7AW7QUg+I9WwZzSTTJ019ENBYr5pV/Q=
	dep	golang.org/x/tools	v0.49.0	h1:3NI7VXzL9+1WZD52Dx2ttoPwD5DWrFGpl9mFZDlmisI=
	build	-buildmode=exe
	build	-compiler=gc
	build	DefaultGODEBUG=cryptocustomrand=1,tlssecpmlkem=0,tracebacklabels=0,urlstrictcolons=0,x509sslcertoverrideplatform=0
	build	CGO_ENABLED=1
	build	CGO_CFLAGS=
	build	CGO_CPPFLAGS=
	build	CGO_CXXFLAGS=
	build	CGO_LDFLAGS=
	build	GOARCH=amd64
	build	GOOS=linux
	build	GOAMD64=v1
```

stderr:

```text

```

### green

stdout:

```text
ok  	github.com/larslarsen/bb-go/modern/accountstore	3.973s
```

stderr:

```text

```

### race

stdout:

```text
ok  	github.com/larslarsen/bb-go/modern/accountauth	7.630s
ok  	github.com/larslarsen/bb-go/modern/accountstore	33.155s
```

stderr:

```text

```

### vet

stdout:

```text

```

stderr:

```text

```

### fuzz

stdout:

```text
fuzz: elapsed: 0s, gathering baseline coverage: 0/15 completed
fuzz: elapsed: 0s, gathering baseline coverage: 15/15 completed, now fuzzing with 2 workers
fuzz: elapsed: 3s, execs: 135009 (45002/sec), new interesting: 3 (total: 18)
fuzz: elapsed: 6s, execs: 270079 (45018/sec), new interesting: 3 (total: 18)
fuzz: elapsed: 9s, execs: 405841 (45256/sec), new interesting: 3 (total: 18)
fuzz: elapsed: 12s, execs: 541224 (45123/sec), new interesting: 3 (total: 18)
fuzz: elapsed: 15s, execs: 674022 (44261/sec), new interesting: 3 (total: 18)
fuzz: elapsed: 18s, execs: 810294 (45435/sec), new interesting: 3 (total: 18)
fuzz: elapsed: 21s, execs: 942823 (44179/sec), new interesting: 3 (total: 18)
fuzz: elapsed: 24s, execs: 1054311 (37161/sec), new interesting: 3 (total: 18)
fuzz: elapsed: 27s, execs: 1170222 (38639/sec), new interesting: 3 (total: 18)
fuzz: elapsed: 30s, execs: 1302391 (44054/sec), new interesting: 3 (total: 18)
fuzz: elapsed: 30s, execs: 1302391 (0/sec), new interesting: 3 (total: 18)
PASS
ok  	github.com/larslarsen/bb-go/modern/accountstore	30.071s
```

stderr:

```text

```

### fault

stdout:

```text
--- FAIL: TestRevocationSurvivesReopen (0.02s)
    persistence_test.go:146: device revocation after reopen = true, <nil>
FAIL
FAIL	github.com/larslarsen/bb-go/modern/accountstore	0.024s
FAIL
```

stderr:

```text

```

### restored

stdout:

```text
ok  	github.com/larslarsen/bb-go/modern/accountstore	0.033s
```

stderr:

```text

```

### gosec

stdout:

```text
Results:


[<repo>/modern/accountstore/store_test.go:923] - G115 (CWE-190): integer overflow conversion int -> uint16 (Confidence: MEDIUM, Severity: HIGH)
    922: 		body := make([]byte, 2+length)
  > 923: 		binary.BigEndian.PutUint16(body[:2], uint16(length))
    924: 		return fixtureSnapshotWithBody(keys.controller, 1, body)

Autofix:

[<repo>/modern/accountstore/store_test.go:158] - G115 (CWE-190): integer overflow conversion int -> uint16 (Confidence: MEDIUM, Severity: HIGH)
    157: 	copy(raw[8:40], controller[:])
  > 158: 	binary.BigEndian.PutUint16(raw[40:42], uint16(count))
    159: 	raw = append(raw, body...)

Autofix:

[<repo>/modern/accountstore/store_test.go:146] - G115 (CWE-190): integer overflow conversion int -> uint16 (Confidence: MEDIUM, Severity: HIGH)
    145: 		var size [2]byte
  > 146: 		binary.BigEndian.PutUint16(size[:], uint16(len(record)))
    147: 		raw = append(raw, size[:]...)

Autofix:

[<repo>/modern/accountstore/store_test.go:143] - G115 (CWE-190): integer overflow conversion int -> uint16 (Confidence: MEDIUM, Severity: HIGH)
    142: 	copy(raw[8:40], controller[:])
  > 143: 	binary.BigEndian.PutUint16(raw[40:42], uint16(count))
    144: 	for _, record := range records {

Autofix:

[<repo>/modern/accountstore/codec.go:49] - G115 (CWE-190): integer overflow conversion int -> uint16 (Confidence: MEDIUM, Severity: HIGH)
    48: 		var length [recordLengthPrefixBytes]byte
  > 49: 		binary.BigEndian.PutUint16(length[:], uint16(len(record)))
    50: 		raw = append(raw, length[:]...)

Autofix:

[<repo>/modern/accountstore/codec.go:49] - G115 (CWE-190): integer overflow conversion int -> uint16 (Confidence: MEDIUM, Severity: HIGH)
    48: 		var length [recordLengthPrefixBytes]byte
  > 49: 		binary.BigEndian.PutUint16(length[:], uint16(len(record)))
    50: 		raw = append(raw, length[:]...)

Autofix:

[<repo>/modern/accountstore/codec.go:46] - G115 (CWE-190): integer overflow conversion int -> uint16 (Confidence: MEDIUM, Severity: HIGH)
    45: 	copy(raw[8:40], controller[:])
  > 46: 	binary.BigEndian.PutUint16(raw[40:42], uint16(len(records)))
    47: 	for _, record := range records {

Autofix:

[<repo>/modern/accountstore/codec.go:46] - G115 (CWE-190): integer overflow conversion int -> uint16 (Confidence: MEDIUM, Severity: HIGH)
    45: 	copy(raw[8:40], controller[:])
  > 46: 	binary.BigEndian.PutUint16(raw[40:42], uint16(len(records)))
    47: 	for _, record := range records {

Autofix:

[<repo>/modern/accountstore/persistence_test.go:605] - G702 (CWE-78): Command injection via taint analysis (Confidence: HIGH, Severity: HIGH)
    604: 	defer cancel()
  > 605: 	cmd := exec.CommandContext(commandCtx, os.Args[0], "-test.run=^TestAcknowledgedRevocationSurvivesProcessKill$", "-test.v")
    606: 	cmd.Env = append(os.Environ(), processKillMarkerEnv+"=1", processKillPathEnv+"="+databasePath)

Autofix:

[<repo>/modern/accountstore/persistence_test.go:605] - G204 (CWE-78): Subprocess launched with a potential tainted input or cmd arguments (Confidence: HIGH, Severity: MEDIUM)
    604: 	defer cancel()
  > 605: 	cmd := exec.CommandContext(commandCtx, os.Args[0], "-test.run=^TestAcknowledgedRevocationSurvivesProcessKill$", "-test.v")
    606: 	cmd.Env = append(os.Environ(), processKillMarkerEnv+"=1", processKillPathEnv+"="+databasePath)

Autofix:

Summary:
  Gosec  : dev
  Files  : 9
  Lines  : 2803
  Nosec  : 0
  Issues : 10
```

stderr:

```text
[gosec] 2026/09/17 12:23:00 Including rules: default
[gosec] 2026/09/17 12:23:00 Excluding rules: default
[gosec] 2026/09/17 12:23:00 Including analyzers: default
[gosec] 2026/09/17 12:23:00 Excluding analyzers: default
[gosec] 2026/09/17 12:23:00 Import directory: <repo>/modern/accountstore
[gosec] 2026/09/17 12:23:01 Checking package: accountstore
[gosec] 2026/09/17 12:23:01 Checking file: <repo>/modern/accountstore/codec.go
[gosec] 2026/09/17 12:23:01 Checking file: <repo>/modern/accountstore/store.go
[gosec] 2026/09/17 12:23:01 Checking file: <repo>/modern/accountstore/types.go
[gosec] 2026/09/17 12:23:01 Checking package: accountstore
[gosec] 2026/09/17 12:23:01 Checking file: <repo>/modern/accountstore/codec.go
[gosec] 2026/09/17 12:23:01 Checking file: <repo>/modern/accountstore/store.go
[gosec] 2026/09/17 12:23:01 Checking file: <repo>/modern/accountstore/types.go
[gosec] 2026/09/17 12:23:01 Checking file: <repo>/modern/accountstore/fuzz_test.go
[gosec] 2026/09/17 12:23:01 Checking file: <repo>/modern/accountstore/persistence_test.go
[gosec] 2026/09/17 12:23:01 Checking file: <repo>/modern/accountstore/store_test.go
[gosec] 2026/09/17 12:23:01 Checking package: main
```

### govulncheck-policy

stdout:

```text

```

stderr:

```text
govulncheck execution failed with exit 1
```

## Review 04 scan completion

Review 04 accepts prior behavior and adjudicates the exact gosec findings.
The temporary fault artifact was archived with identical bytes as store-fault.go.txt.
Prior metadata/overlay are historical and retain the original filename.

Capture: `modern/dist/acc002/publication-20260917T205446828868Z`.
Dependency gate satisfied: `True`.
Staged secret scan and Git evidence retained in publication.json.
The report is frozen before that scan; publication conditional.

```json
{
  "started_utc": "2026-09-17T20:54:46.828993+00:00",
  "filesystem": "ext4",
  "environment": {
    "HOME": "<home>",
    "LANG": "C.UTF-8",
    "PATH": "<home>/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.linux-amd64/bin:<home>/go/bin:/usr/bin:/bin",
    "GOWORK": "off",
    "GOTOOLCHAIN": "local",
    "GOENV": "off",
    "GOPROXY": "off",
    "GOSUMDB": "off",
    "GOFLAGS": "-mod=readonly -p=2",
    "GOMAXPROCS": "2",
    "CGO_ENABLED": "1",
    "GOTELEMETRY": "off",
    "GOMODCACHE": "<home>/go/pkg/mod",
    "GOCACHE": "<repo>/modern/dist/acc002/green-20260917T192111797366Z/gocache",
    "TMPDIR": "<repo>/modern/dist/acc002/green-20260917T192111797366Z/tmp",
    "XDG_CACHE_HOME": "<repo>/modern/dist/acc002/green-20260917T192111797366Z/xdg-cache",
    "PYTHONDONTWRITEBYTECODE": "1"
  },
  "before": {
    "modern/accountauth/types.go": "ac264e0d967543f915d8a305eda7a78e470dcb6146b2048305fc10070fe98dfb",
    "modern/accountauth/records.go": "cafbb6b14c7fab54df0d14e9cf5822c85f8d0d9e5a98c2aa68c56ea705aa0640",
    "modern/accountauth/state.go": "b1807c69ffaf155dd2a4fd8f1ba54ba58bdaa85c2519c84adaabecd8ee48bff1",
    "modern/accountauth/records_test.go": "264aa1b384ce201dc5d69f92488d2ec0807de59ef048b46118f0f96608dd8fe9",
    "modern/accountauth/state_test.go": "949066a95bd9a8cda7b0bcedbbd3bea25f36185f9b16db6f48849638dbeef9ed",
    "modern/accountauth/fuzz_test.go": "08245bcfdc8d2620f0499c69c2eacdd2033c47beec68959b080bd2d9f960b833",
    "modern/go.mod": "1150b94372852355beaffa7104430a21e8f8aa6ec4877bad18d0bcdb71453783",
    "modern/go.sum": "4c91209822dccd4a60955ddd6b8b94a327e88b55721577494c953a705395b83a",
    "modern/network/open.go": "96bf07274832c57ef67ace9a6e0a5dc06651bc7a5b27eef7cff7f480610d4f3b",
    "modern/accountstore/store_test.go": "6c7c3b4c0849670fc4b1527091b0fb40e26854db10013813c885f5cff5940e32",
    "modern/accountstore/persistence_test.go": "25e8308df9ff6a6aa3d1e43747a6d9a09efd66c12d36a6c3e19e1ef3db9e4c3a",
    "modern/accountstore/fuzz_test.go": "ecff630ee50e23b4651d34aab7190f4872b2f8ad2e0d91b718e31d3518bcc9af",
    "modern/accountstore/types.go": "d10523e39dc4b0e40b80be7105094c1df0d514dd3e156509e47de9a753f7b24f",
    "modern/accountstore/store.go": "7bd1a76b1055bcaa1723f751c702aa70fd4e6d40a6947267c3c1a6c1ab9b6d81",
    "modern/accountstore/codec.go": "fe864efcf92f2f2168da37fa7ea3b482b35f0463e438023dad27a10470e0deab"
  },
  "tools": {
    "<home>/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.linux-amd64/bin/go": "1db869c560a193573a71be466a34e0d4abb7792d78165c6102cdda069276a3a8",
    "<home>/go/bin/gosec": "eb00a1fb095b161a48c5bcadbe1e246bbafe270da497a122d2e63ade346954c2",
    "<home>/go/bin/govulncheck": "6c92f0536311f5e2083a839c75558e3fb986758a320a402aa8f524c85ffd7400",
    "<repo>/scripts/govulncheck_policy.py": "709cb00d44c62ef6e2d394f457407183d6fb90bc98958c80db0261607bc3c77c",
    "<home>/OpenBazaar/.security-tools/bbgo-sec-tools-20260829/gitleaks": "444a87409b36e0c330caf3fa61f354dd13e66987ecc9db63d787db761641541a"
  },
  "commands": [
    {
      "name": "dht-diversity",
      "argv": [
        "<home>/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.linux-amd64/bin/go",
        "test",
        "./network",
        "-run",
        "^TestDHTRoutingTableEnforcesIPDiversity$",
        "-count=1",
        "-timeout=3m"
      ],
      "cwd": "<repo>/modern",
      "started_utc": "2026-09-17T20:54:46.848506+00:00",
      "returncode": 0,
      "finished_utc": "2026-09-17T20:55:10.904742+00:00",
      "stdout_sha256": "26c4cd4cc27d79e233836556e659f4df494a8b321b9b14c43de3e5c98712f026",
      "stderr_sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
      "after": {
        "modern/accountauth/types.go": "ac264e0d967543f915d8a305eda7a78e470dcb6146b2048305fc10070fe98dfb",
        "modern/accountauth/records.go": "cafbb6b14c7fab54df0d14e9cf5822c85f8d0d9e5a98c2aa68c56ea705aa0640",
        "modern/accountauth/state.go": "b1807c69ffaf155dd2a4fd8f1ba54ba58bdaa85c2519c84adaabecd8ee48bff1",
        "modern/accountauth/records_test.go": "264aa1b384ce201dc5d69f92488d2ec0807de59ef048b46118f0f96608dd8fe9",
        "modern/accountauth/state_test.go": "949066a95bd9a8cda7b0bcedbbd3bea25f36185f9b16db6f48849638dbeef9ed",
        "modern/accountauth/fuzz_test.go": "08245bcfdc8d2620f0499c69c2eacdd2033c47beec68959b080bd2d9f960b833",
        "modern/go.mod": "1150b94372852355beaffa7104430a21e8f8aa6ec4877bad18d0bcdb71453783",
        "modern/go.sum": "4c91209822dccd4a60955ddd6b8b94a327e88b55721577494c953a705395b83a",
        "modern/network/open.go": "96bf07274832c57ef67ace9a6e0a5dc06651bc7a5b27eef7cff7f480610d4f3b",
        "modern/accountstore/store_test.go": "6c7c3b4c0849670fc4b1527091b0fb40e26854db10013813c885f5cff5940e32",
        "modern/accountstore/persistence_test.go": "25e8308df9ff6a6aa3d1e43747a6d9a09efd66c12d36a6c3e19e1ef3db9e4c3a",
        "modern/accountstore/fuzz_test.go": "ecff630ee50e23b4651d34aab7190f4872b2f8ad2e0d91b718e31d3518bcc9af",
        "modern/accountstore/types.go": "d10523e39dc4b0e40b80be7105094c1df0d514dd3e156509e47de9a753f7b24f",
        "modern/accountstore/store.go": "7bd1a76b1055bcaa1723f751c702aa70fd4e6d40a6947267c3c1a6c1ab9b6d81",
        "modern/accountstore/codec.go": "fe864efcf92f2f2168da37fa7ea3b482b35f0463e438023dad27a10470e0deab"
      }
    },
    {
      "name": "govulncheck-policy",
      "argv": [
        "/usr/bin/python3",
        "-c",
        "import datetime, json, pathlib, runpy, subprocess, sys\nrun = pathlib.Path(sys.argv[2])\nns = runpy.run_path(sys.argv[1], run_name='acc002_policy')\ndef execute(argv, **kwargs):\n    utc = lambda: datetime.datetime.now(datetime.timezone.utc).isoformat()\n    entry = {'argv': argv, 'cwd': kwargs.get('cwd'), 'started_utc': utc()}\n    result = subprocess.run(argv, **kwargs)\n    (run / 'govulncheck.sarif').write_text(result.stdout or '')\n    (run / 'govulncheck.stderr').write_text(result.stderr or '')\n    entry.update(returncode=result.returncode, finished_utc=utc())\n    (run / 'govulncheck-command.json').write_text(json.dumps(entry, indent=2))\n    return result\nraise SystemExit(ns['main'](['source'], execute=execute))\n",
        "<repo>/scripts/govulncheck_policy.py",
        "<repo>/modern/dist/acc002/publication-20260917T205446828868Z"
      ],
      "cwd": "<repo>",
      "started_utc": "2026-09-17T20:55:10.908116+00:00",
      "returncode": 0,
      "finished_utc": "2026-09-17T20:55:22.444350+00:00",
      "stdout_sha256": "05234cec3775b7be8a52ab2326ce83935c9f60cc4e78ae00d016dafd5e6bbf7d",
      "stderr_sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
      "after": {
        "modern/accountauth/types.go": "ac264e0d967543f915d8a305eda7a78e470dcb6146b2048305fc10070fe98dfb",
        "modern/accountauth/records.go": "cafbb6b14c7fab54df0d14e9cf5822c85f8d0d9e5a98c2aa68c56ea705aa0640",
        "modern/accountauth/state.go": "b1807c69ffaf155dd2a4fd8f1ba54ba58bdaa85c2519c84adaabecd8ee48bff1",
        "modern/accountauth/records_test.go": "264aa1b384ce201dc5d69f92488d2ec0807de59ef048b46118f0f96608dd8fe9",
        "modern/accountauth/state_test.go": "949066a95bd9a8cda7b0bcedbbd3bea25f36185f9b16db6f48849638dbeef9ed",
        "modern/accountauth/fuzz_test.go": "08245bcfdc8d2620f0499c69c2eacdd2033c47beec68959b080bd2d9f960b833",
        "modern/go.mod": "1150b94372852355beaffa7104430a21e8f8aa6ec4877bad18d0bcdb71453783",
        "modern/go.sum": "4c91209822dccd4a60955ddd6b8b94a327e88b55721577494c953a705395b83a",
        "modern/network/open.go": "96bf07274832c57ef67ace9a6e0a5dc06651bc7a5b27eef7cff7f480610d4f3b",
        "modern/accountstore/store_test.go": "6c7c3b4c0849670fc4b1527091b0fb40e26854db10013813c885f5cff5940e32",
        "modern/accountstore/persistence_test.go": "25e8308df9ff6a6aa3d1e43747a6d9a09efd66c12d36a6c3e19e1ef3db9e4c3a",
        "modern/accountstore/fuzz_test.go": "ecff630ee50e23b4651d34aab7190f4872b2f8ad2e0d91b718e31d3518bcc9af",
        "modern/accountstore/types.go": "d10523e39dc4b0e40b80be7105094c1df0d514dd3e156509e47de9a753f7b24f",
        "modern/accountstore/store.go": "7bd1a76b1055bcaa1723f751c702aa70fd4e6d40a6947267c3c1a6c1ab9b6d81",
        "modern/accountstore/codec.go": "fe864efcf92f2f2168da37fa7ea3b482b35f0463e438023dad27a10470e0deab"
      }
    }
  ],
  "base_commit": "f5aa3f4b91fc2cf6bd0d8f2244e7f0c18dd464b5",
  "artifact_relocation": {
    "from": "<repo>/modern/dist/acc002/green-20260917T192111797366Z/store-fault.go",
    "to": "<repo>/modern/dist/acc002/green-20260917T192111797366Z/store-fault.go.txt",
    "sha256": "f9f5b359b8f7414d28cc9bfcf45b35cd3236735a03e4767615fc8468480e2849"
  },
  "dependency_gate": true
}
```

### dht-diversity

stdout:

```text
ok  	github.com/larslarsen/bb-go/modern/network	0.059s
```

stderr:

```text

```

### govulncheck-policy

stdout:

```text
Govulncheck source scan: accepted reviewed exception
Accepted reviewed exception GO-2024-3218 on github.com/libp2p/go-libp2p-kad-dht@v0.42.2
owner: Lead Engineer/Reviewer — Codex
expires: 2026-11-29
error results: 1
warning results: 0
note results: 4
- error GO-2024-3218: Your code calls vulnerable functions in 11 packages (github.com/libp2p/go-libp2p-kad-dht, github.com/libp2p/go-libp2p-kad-dht/amino, github.com/libp2p/go-libp2p-kad-dht/internal, github.com/libp2p/go-libp2p-kad-dht/internal/config, github.com/libp2p/go-libp2p-kad-dht/internal/metrics, github.com/libp2p/go-libp2p-kad-dht/internal/net, github.com/libp2p/go-libp2p-kad-dht/netsize, github.com/libp2p/go-libp2p-kad-dht/pb, github.com/libp2p/go-libp2p-kad-dht/qpeerset, github.com/libp2p/go-libp2p-kad-dht/records, and github.com/libp2p/go-libp2p-kad-dht/rtrefresh).
- note GO-2026-5932: Your code depends on 1 vulnerable module (golang.org/x/crypto), but doesn't appear to call any of the vulnerable symbols.
- note GO-2026-6303: Your code depends on 1 vulnerable module (golang.org/x/crypto), but doesn't appear to call any of the vulnerable symbols.
- note GO-2026-6354: Your code depends on 1 vulnerable module (golang.org/x/crypto), but doesn't appear to call any of the vulnerable symbols.
- note GO-2026-6355: Your code depends on 1 vulnerable module (golang.org/x/crypto), but doesn't appear to call any of the vulnerable symbols.
```

stderr:

```text

```
