"""저장소 공용 Bazel 매크로."""

# Bazel 라벨에 쓸 수 있는 문자만 허용한다. 공백·한글 파일명은 target-determinator 의 라벨 파서가 거부한다.
_SAFE_CHARS = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789/._-+=,@~"

def _is_safe(path):
    for c in path.elems():
        if c not in _SAFE_CHARS:
            return False
    return True

def lab_files(name = "files"):
    """코드가 없는 lab 의 파일 묶음. `bazel query //...` 에 lab 이 보이게 한다. 라벨로 쓸 수 없는 이름의 파일은 뺀다."""
    native.filegroup(
        name = name,
        srcs = [f for f in native.glob(["**"], exclude = ["BUILD.bazel"]) if _is_safe(f)],
    )
