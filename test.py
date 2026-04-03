import requests
import threading
import time
import os
import hashlib

# Configuration
BASE_URL = "http://localhost:8080/api/v1"
TOKEN = "YOUR_JWT_TOKEN_HERE" # Replace with your real JWT
FILE_PATH = "test_large_file.bin"
FILE_SIZE_MB = 512 # 512MB for OOM check
CHUNK_SIZE = 10 * 1024 * 1024 # 10MB per chunk

HEADERS = {"Authorization": f"Bearer {TOKEN}"}

def generate_test_file(path, size_mb):
    # Create a large file with random content
    with open(path, "wb") as f:
        f.write(os.urandom(size_mb * 1024 * 1024))
    print(f"File generated: {path} ({size_mb} MB)")

def get_file_hash(path):
    sha256_hash = hashlib.sha256()
    with open(path, "rb") as f:
        for byte_block in iter(lambda: f.read(4096), b""):
            sha256_hash.update(byte_block)
    return sha256_hash.hexdigest()

def run_test():
    # 1. Generate file
    generate_test_file(FILE_PATH, FILE_SIZE_MB)
    file_size = os.path.getsize(FILE_PATH)
    file_name = os.path.basename(FILE_PATH)

    # 2. Init Upload
    print("Step 1: Initializing Upload...")
    init_data = {
        "file_name": file_name,
        "total_size": file_size,
        "chunk_size": CHUNK_SIZE
    }
    resp = requests.post(f"{BASE_URL}/upload/init", data=init_data, headers=HEADERS)
    if resp.status_code != 200:
        print(f"Init Failed: {resp.text}")
        return
    upload_id = resp.json().get("upload_id")
    print(f"UploadID: {upload_id}")

    # 3. Upload Parts (Streaming test)
    print("Step 2: Uploading Parts...")
    with open(FILE_PATH, "rb") as f:
        part_num = 1
        while True:
            chunk = f.read(CHUNK_SIZE)
            if not chunk: break
            # Put request with stream data
            url = f"{BASE_URL}/upload/{upload_id}/part/{part_num}"
            files = {'part': ('chunk.bin', chunk)} 
            p_resp = requests.put(url, files=files, headers=HEADERS)
            if p_resp.status_code != 200:
                print(f"Part {part_num} Failed: {p_resp.status_code} - {p_resp.text}")
            part_num += 1
    print("All parts uploaded.")

    # 4. Concurrent Complete (Race Condition Test)
    print("Step 3: Triggering Concurrent Complete (Redis Lock Test)...")
    results = []
    def call_complete():
        url = f"{BASE_URL}/upload/{upload_id}/complete"
        res = requests.post(url, headers=HEADERS)
        results.append(res.status_code)
        if res.status_code != 200:
            print(f"Merge Failed: {res.text}")

    threads = []
    for _ in range(5): # Simulate 5 concurrent merge requests
        t = threading.Thread(target=call_complete)
        threads.append(t)
        t.start()
    
    for t in threads: t.join()

    # Verify Redis Lock: Expect only one 200, others 409 or 500 (depending on your logic)
    success_count = results.count(200)
    print(f"Complete results: {results}")
    print(f"Success merges: {success_count} (Expected: 1)")

if __name__ == "__main__":
    run_test()