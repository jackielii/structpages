# Performance Optimizations

This document tracks performance optimizations and future improvement opportunities.

## Implemented Optimizations (v0.x.x - 2025-10-31)

### 1. Cache extractMethodInfo Results
- **Impact:** 80-89% faster reflection operations
- **Implementation:** Global `sync.Map` cache for method info
- **Benefit:** Zero allocations for unbound method lookups

### 2. Cache parseSegments Results
- **Impact:** 10-17% faster URL generation
- **Implementation:** Per-context cache with RWMutex
- **Benefit:** Eliminated redundant pattern parsing

### 3. Pre-parse Route Segments
- **Impact:** 3% faster request handling with params
- **Implementation:** Store parsed segments in PageNode at Mount time
- **Benefit:** Zero parsing overhead in extractURLParams middleware

## Benchmarks

Run benchmarks to verify performance:

```bash
# Full benchmark suite
go test -bench=. -benchmem

# Specific categories
go test -bench=BenchmarkRequestHandling -benchmem
go test -bench=BenchmarkURLGeneration -benchmem
go test -bench=BenchmarkIDGeneration -benchmem
go test -bench=BenchmarkReflection -benchmem
```

## Future Optimization Opportunities

### Medium Priority

1. **Pool availableArgs Maps**
   - Use `sync.Pool` for argument maps in method calling
   - Estimated improvement: 5-10% in request handling

2. **Optimize kebabToPascal**
   - Single-pass implementation without `strings.Split`
   - Estimated improvement: 50% in HTMX target matching

### Low Priority

3. **Cache Component Results**
   - For pure components with no side effects
   - Requires careful API design

4. **Reduce Context Value() Calls**
   - Extract once at handler start
   - Pass as struct instead of multiple lookups

## Performance Characteristics

Current performance (Apple M3 Ultra):
- Simple GET request: ~760ns
- Request with URL params: ~1050ns
- URL generation (1 param): ~226ns
- ID generation (unbound): ~167ns
- extractMethodInfo (cached): ~8ns

Memory per request:
- Simple GET: 1466B, 16 allocs
- With params: 2188B, 20 allocs
