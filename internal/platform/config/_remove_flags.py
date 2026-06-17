with open('config.go', 'r') as f:
    lines = f.readlines()

new_lines = []
for l in lines:
    s = l.strip()
    # Remove lines that are bool flag assignments inside the RedisFeatures initializer
    if s.endswith(',') and ('CacheEnabled' in s or 'IdempotencyEnabled' in s or 'ReadinessCacheEnabled' in s):
        continue
    new_lines.append(l)

with open('config.go', 'w') as f:
    f.writelines(new_lines)

print(f'Removed {len(lines) - len(new_lines)} lines')
