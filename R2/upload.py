import os
import hashlib
import sqlite3
import boto3
import threading
from mimetypes import guess_type
from concurrent.futures import ThreadPoolExecutor, as_completed

# --- 配置 ---
DB_PATH = "games_assets.db"
OSS_CONFIG = {
    "bucket": "test",
    "endpoint": "https://e81cd9a68fe7ea7e3d6b46fe7f5cbd3c.r2.cloudflarestorage.com",
    "ak": "3423adc1bfdfde0b248ef1f946b4aa75",
    "sk": "14e469bd1bd78a278a8bd362c23bdbb365e80b81a14389087788ec1666e03b8b",
    "url_prefix": "https://dlsite.810114.xyz",
}
MAX_WORKERS = 1000  # 并发线程数，根据你的带宽建议设置在 5-20 之间

# 全局锁，用于保护数据库写入
# db_lock = threading.Lock()


# def get_hash(file_path):
#     h = hashlib.md5()
#     try:
#         with open(file_path, "rb") as f:
#             for chunk in iter(lambda: f.read(8192), b""):
#                 h.update(chunk)
#         return h.hexdigest()
#     except Exception as e:
#         print(f"[错误] 哈希计算失败: {file_path}, {e}")
#         return None


# def init_db():
#     # check_same_thread=False 允许在多线程中共享连接对象（但写入仍需加锁）
#     conn = sqlite3.connect(DB_PATH, check_same_thread=False)
#     conn.execute(
#         "CREATE TABLE IF NOT EXISTS files (game_id TEXT, path TEXT, hash TEXT, PRIMARY KEY(game_id, path))"
#     )
#     conn.execute("CREATE TABLE IF NOT EXISTS blobs (hash TEXT PRIMARY KEY, url TEXT)")
#     conn.commit()
#     return conn


def process_single_file(s3_client, conn, game_id, local_dir, rel_path):
    try:
        local_path = os.path.join(local_dir, rel_path)
        # file_hash = get_hash(local_path)
        # if not file_hash:
        #     return

        # OSS 存储路径直接用: game_id/rel_path
        object_key = f"{game_id}/{rel_path}"
        target_url = f"{OSS_CONFIG['url_prefix']}/{object_key}"

        # 1. 检查是否已经记录过该文件映射 (为了避免重复上传)
        # 注意：这里我们检查 game_id + path，而不是全局 hash
        # with db_lock:
        #     res = conn.execute(
        #         "SELECT hash FROM files WHERE game_id = ? AND path = ?", (game_id, rel_path)
        #     ).fetchone()

        if True:
            # 文件不存在，或者哈希变了，执行上传
            mime_type, _ = guess_type(local_path)
            s3_client.upload_file(
                local_path,
                OSS_CONFIG["bucket"],
                object_key,
                ExtraArgs={"ContentType": mime_type or "application/octet-stream"},
            )
            
            # with db_lock:
            #     # 更新 blobs 表 (存一下 hash 和 url 备查)
            #     conn.execute(
            #         "INSERT OR REPLACE INTO blobs (hash, url) VALUES (?, ?)",
            #         (file_hash, target_url),
            #     )
            #     # 更新路径映射表
            #     conn.execute(
            #         "INSERT OR REPLACE INTO files (game_id, path, hash) VALUES (?, ?, ?)",
            #         (game_id, rel_path, file_hash),
            #     )
        return True
    except Exception as e:
        print(f"[失败] 文件 {rel_path}: {e}")
        return False


def upload_game(game_id, local_dir):
    if not os.path.exists(local_dir):
        print(f"目录不存在: {local_dir}")
        return

    # conn = init_db()
    s3 = boto3.client(
        "s3",
        endpoint_url=OSS_CONFIG["endpoint"],
        aws_access_key_id=OSS_CONFIG["ak"],
        aws_secret_access_key=OSS_CONFIG["sk"],
        region_name="auto",
    )

    # 1. 先扫描所有需要处理的文件
    print("[*] 正在扫描本地文件...")
    tasks = []
    for root, _, files in os.walk(local_dir):
        for file in files:
            if file.endswith((".rpgsave", ".dll", ".exe", ".bin", ".dat", ".pak", ".info")):
                continue

            local_path = os.path.join(root, file)
            rel_path = os.path.relpath(local_path, local_dir).replace("\\", "/")
            tasks.append(rel_path)

    total_files = len(tasks)
    print(
        f"[*] 扫描完成，共 {total_files} 个文件待处理。开始并行上传 (线程数: {MAX_WORKERS})..."
    )

    # 2. 使用线程池并行处理
    done_count = 0
    with ThreadPoolExecutor(max_workers=MAX_WORKERS) as executor:
        # 提交所有任务
        future_to_file = {
            executor.submit(
                process_single_file, s3, None, game_id, local_dir, path
            ): path
            for path in tasks
        }

        for future in as_completed(future_to_file):
            path = future_to_file[future]
            try:
                result = future.result()
                if result:
                    done_count += 1
            except Exception as e:
                print(f"[异常] 线程执行出错 {path}: {e}")

            if done_count % 20 == 0 or done_count == total_files:
                print(
                    f"进度: {done_count}/{total_files} ({(done_count/total_files)*100:.1f}%)"
                )

    # 3. 最后提交一次数据库
    # conn.commit()
    # conn.close()
    print(f"\n[完成] 所有任务处理完毕，成功处理 {done_count} 个文件。")


if __name__ == "__main__":
    TARGET_PATH = r"/mnt/c/Users/lumin/Downloads/3989/退魔師朱里/www"
    GAME_ID = "RJ01202661"

    try:
        upload_game(GAME_ID, TARGET_PATH)
    except KeyboardInterrupt:
        print("\n[用户中断] 正在停止...")
    except Exception as e:
        import traceback

        traceback.print_exc()

    input("\n处理结束，按回车退出...")
