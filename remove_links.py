#!/usr/bin/env python3
import os
import re

def remove_changelog_links():
    # 遍历当前目录及子目录
    for root, dirs, files in os.walk('.'):
        for file in files:
            if file == 'CHANGELOG.md':
                file_path = os.path.join(root, file)
                try:
                    with open(file_path, 'r', encoding='utf-8') as f:
                        content = f.read()
                    
                    # 使用正则表达式替换标题链接
                    # 匹配格式: ## [版本号](链接) (日期)
                    # 替换为: ## 版本号 (日期)
                    pattern = r'## \[([^\]]+)\]\([^)]+\) (\([^)]+\))'
                    replacement = r'## \1 \2'
                    new_content = re.sub(pattern, replacement, content)
                    
                    # 如果内容有变化，写回文件
                    if new_content != content:
                        with open(file_path, 'w', encoding='utf-8') as f:
                            f.write(new_content)
                        print(f'已移除链接: {file_path}')
                    else:
                        print(f'无需修改: {file_path}')
                        
                except Exception as e:
                    print(f'处理文件 {file_path} 时出错: {e}')

if __name__ == '__main__':
    remove_changelog_links() 