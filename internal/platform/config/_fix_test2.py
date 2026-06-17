with open('config_test.go', 'r') as f:
    content = f.read()

# Add back the WORKER_ENABLED_CONSUMERS line that was accidentally removed
old = '\t\tt.Setenv("WORKER_CONCURRENCY", "3")'
new = '\t\tt.Setenv("WORKER_ENABLED_CONSUMERS", "delivery, analytics, ")\n\t\tt.Setenv("WORKER_CONCURRENCY", "3")'

if old in content:
    content = content.replace(old, new)
    print('SUCCESS')
else:
    print('FAIL: pattern not found')
    # Try with single tab
    old2 = '\t\tt.Setenv("WORKER_CONCURRENCY"'
    idx = content.find(old2)
    if idx >= 0:
        print('Found at', idx)
        print(repr(content[idx:idx+50]))

with open('config_test.go', 'w') as f:
    f.write(content)
