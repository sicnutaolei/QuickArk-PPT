import os
import shutil

def extract_chapter_section(filename):
    # 提取章节信息
    if '第一章' in filename:
        chapter = '1'
    elif '第二章' in filename:
        chapter = '2'
    elif '第三章' in filename:
        chapter = '3'
    elif '第四章' in filename:
        chapter = '4'
    elif '第五章' in filename:
        chapter = '5'
    else:
        return None, None
    
    # 提取节信息
    if '第一节' in filename:
        section = '1'
    elif '第二节' in filename:
        section = '2'
    elif '第三节' in filename:
        section = '3'
    elif '第四节' in filename:
        section = '4'
    elif '第五节' in filename:
        section = '5'
    elif '整理与提升' in filename:
        section = '整理与提升'
    elif '复习课' in filename:
        section = '复习课'
    elif '提升课' in filename:
        section = '提升课'
    else:
        return chapter, None
    
    return chapter, section

def main():
    current_dir = os.getcwd()
    files = [f for f in os.listdir(current_dir) if f.endswith('.pptx')]
    
    for file in files:
        chapter, section = extract_chapter_section(file)
        if chapter:
            # 创建目录结构
            chapter_dir = f'第{chapter}章'
            if section:
                section_dir = os.path.join(chapter_dir, f'第{section}节' if section not in ['整理与提升', '复习课', '提升课'] else section)
            else:
                section_dir = chapter_dir
            
            if not os.path.exists(chapter_dir):
                os.makedirs(chapter_dir)
            if not os.path.exists(section_dir):
                os.makedirs(section_dir)
            
            # 移动文件
            src_path = os.path.join(current_dir, file)
            dst_path = os.path.join(section_dir, file)
            shutil.move(src_path, dst_path)
            print(f'已移动: {file} -> {section_dir}')
        else:
            print(f'无法解析: {file}')

if __name__ == '__main__':
    main()
