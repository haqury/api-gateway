#!/bin/sh
set -e

echo "🔧 Protoc Go Builder for Shared Library"
echo "==========================================="

# Пути
PROTO_ROOT="pkg/proto"
OUTPUT_DIR="internal/gen"
INCLUDE_DIRS="-I ${PROTO_ROOT} -I /include"

# Если переданы аргументы, используем их
if [ $# -gt 0 ]; then
    echo "Custom command: $@"
    exec "$@"
else
    # Автоматическая генерация
    echo "Proto root: ${PROTO_ROOT}"
    echo "Output dir: ${OUTPUT_DIR}"
    echo ""

    # Создаем выходную директорию
    mkdir -p ${OUTPUT_DIR}

    # Ищем все proto файлы рекурсивно
    find ${PROTO_ROOT} -name "*.proto" | while read proto_file; do
        # Определяем путь относительно proto root
        rel_path="${proto_file#${PROTO_ROOT}/}"
        dir_path="$(dirname ${rel_path})"

        echo "📁 Processing: ${rel_path}"

        # Создаем соответствующую структуру в выходной директории
        mkdir -p ${OUTPUT_DIR}/${dir_path}

        # Генерируем код
        protoc ${INCLUDE_DIRS} \
            --go_out=${OUTPUT_DIR} \
            --go_opt=paths=source_relative \
            --go-grpc_out=${OUTPUT_DIR} \
            --go-grpc_opt=paths=source_relative \
            "${proto_file}"

        if [ $? -eq 0 ]; then
            echo "  ✅ Generated: ${dir_path}/"
        else
            echo "  ❌ Failed: ${rel_path}"
            exit 1
        fi
    done

    echo ""
    echo "✅ All proto files generated!"
    echo "📁 Output: ${OUTPUT_DIR}/"
fi