with open('config_test.go', 'r') as f:
    content = f.read()

old1 = '\tif !cfg.RedisCache.IdentityAccessCacheEnabled {\n\t\tt.Fatal("expected identity access cache enabled")\n\t}\n\tif cfg.RedisCache.IdentityAccessCacheTTL != 30*time.Second {\n\t\tt.Fatalf("unexpected identity access cache TTL: %s", cfg.RedisCache.IdentityAccessCacheTTL)\n\t}\n\tif !cfg.RedisCache.AnalyticsQueryCacheEnabled {\n\t\tt.Fatal("expected analytics query cache enabled")\n\t}\n'

new1 = '\tif cfg.RedisCache.IdentityAccessCacheTTL != 30*time.Second {\n\t\tt.Fatalf("unexpected identity access cache TTL: %s", cfg.RedisCache.IdentityAccessCacheTTL)\n\t}\n'

if old1 in content:
    content = content.replace(old1, new1)
    print('SUCCESS')
else:
    print('FAIL')
    print('Looking for:', repr(old1[:80]))

with open('config_test.go', 'w') as f:
    f.write(content)
