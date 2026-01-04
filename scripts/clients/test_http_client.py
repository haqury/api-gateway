#!/usr/bin/env python3
"""
HTTP клиент для тестирования API Gateway
Поддерживает два режима:
1. JSON с base64 (старый)
2. Multipart с бинарными данными (новый)
"""

import requests
import json
import base64
import time
import sys
from pathlib import Path

API_BASE = "http://localhost:8080/api/v1"

def test_health():
    """Проверка доступности сервиса"""
    try:
        resp = requests.get(f"{API_BASE}/../health", timeout=5)
        return resp.status_code == 200
    except:
        return False

def test_base64_mode():
    """Тест старого режима (base64 в JSON)"""
    print("Testing base64 mode...")
    
    # Создаем тестовый стрим
    stream_req = {
        "client_id": "test_python_client",
        "user_id": "python_user",
        "camera_name": "python_camera",
        "filename": "test.mp4"
    }
    
    resp = requests.post(f"{API_BASE}/video/start", json=stream_req)
    if resp.status_code != 200:
        print(f"Failed to start stream: {resp.text}")
        return False
    
    stream_data = resp.json()
    stream_id = stream_data.get("stream_id")
    print(f"Stream started: {stream_id}")
    
    # Отправляем тестовый кадр (миниатюрное base64 изображение)
    test_image = base64.b64encode(b"fake_image_data").decode('utf-8')
    
    frame_req = {
        "stream_id": stream_id,
        "client_id": "test_python_client",
        "user_name": "Python User",
        "frame": {
            "frame_data": test_image,
            "timestamp": int(time.time()),
            "width": 1920,
            "height": 1080,
            "format": "jpeg"
        }
    }
    
    resp = requests.post(f"{API_BASE}/video/frame", json=frame_req)
    if resp.status_code != 200:
        print(f"Failed to send frame: {resp.text}")
        return False
    
    print("Frame sent successfully via base64")
    
    # Получаем статистику
    stats_resp = requests.get(f"{API_BASE}/video/stats/{stream_id}")
    if stats_resp.status_code == 200:
        print(f"Stats: {stats_resp.json()}")
    
    # Останавливаем стрим
    stop_req = {
        "stream_id": stream_id,
        "client_id": "test_python_client",
        "filename": "test.mp4"
    }
    
    resp = requests.post(f"{API_BASE}/video/stop", json=stop_req)
    print(f"Stream stopped: {resp.json()}")
    
    return True

def test_multipart_mode():
    """Тест нового режима (multipart с бинарными данными)"""
    print("\nTesting multipart mode...")
    
    # Подготовка тестовых данных
    metadata = {
        "stream_id": f"multipart_test_{int(time.time())}",
        "client_id": "python_multipart",
        "user_name": "Multipart User",
        "timestamp": str(int(time.time()))
    }
    
    # Создаем тестовый бинарный файл
    test_data = b"fake_binary_video_data" * 100  # 2.4KB
    
    # Отправляем multipart запрос
    files = {
        'frame': ('frame.bin', test_data, 'application/octet-stream')
    }
    
    data = {
        'metadata': json.dumps(metadata)
    }
    
    try:
        resp = requests.post(
            f"{API_BASE}/video/frame",
            files=files,
            data=data,
            timeout=10
        )
        
        if resp.status_code == 200:
            print("✅ Multipart frame sent successfully")
            print(f"Response: {resp.json()}")
            return True
        else:
            print(f"❌ Multipart failed: {resp.status_code} - {resp.text}")
            return False
            
    except Exception as e:
        print(f"❌ Multipart error: {e}")
        return False

def test_auto_stream():
    """Тест автокреации стрима"""
    print("\nTesting auto-stream creation...")
    
    resp = requests.post(f"{API_BASE}/test/auto-stream", json={
        "client_id": "auto_test",
        "user_id": "auto_user",
        "camera": "auto_camera"
    })
    
    if resp.status_code == 200:
        data = resp.json()
        print(f"✅ Auto-stream created: {data.get('stream_id')}")
        print("Instructions:", data.get('instructions'))
        return True
    else:
        print(f"❌ Auto-stream failed: {resp.text}")
        return False

def main():
    """Основная функция тестирования"""
    print("=" * 50)
    print("HTTP Client Test for API Gateway")
    print("Dual API Mode (HTTP + gRPC)")
    print("=" * 50)
    
    # Проверяем доступность сервиса
    if not test_health():
        print("❌ Service is not available. Start the server first.")
        print("Run: make run")
        sys.exit(1)
    
    print("✅ Service is running")
    
    # Запускаем тесты
    tests = [
        ("Auto-stream creation", test_auto_stream),
        ("Base64 mode", test_base64_mode),
        ("Multipart mode", test_multipart_mode),
    ]
    
    results = []
    for test_name, test_func in tests:
        print(f"\n{'='*30}")
        print(f"Test: {test_name}")
        print(f"{'='*30}")
        
        try:
            success = test_func()
            results.append((test_name, success))
            time.sleep(1)  # Пауза между тестами
        except Exception as e:
            print(f"❌ Test crashed: {e}")
            results.append((test_name, False))
    
    # Вывод результатов
    print(f"\n{'='*50}")
    print("TEST RESULTS:")
    print(f"{'='*50}")
    
    for test_name, success in results:
        status = "✅ PASS" if success else "❌ FAIL"
        print(f"{status} - {test_name}")
    
    # Общая статистика
    passed = sum(1 for _, success in results if success)
    total = len(results)
    
    print(f"\nTotal: {passed}/{total} tests passed")
    
    if passed == total:
        print("\n🎉 All tests passed! Dual API is working correctly.")
    else:
        print("\n⚠ Some tests failed. Check the server logs.")
        sys.exit(1)

if __name__ == "__main__":
    main()
