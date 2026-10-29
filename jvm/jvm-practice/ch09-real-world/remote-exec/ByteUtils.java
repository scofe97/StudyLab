// 바이트 배열 ↔ 정수 변환 + 부분 교체. 상수 풀 length 필드(2바이트)를 읽고 쓰는 데 쓴다.
// 핵심: 자바 byte 는 부호 있음 → & 0xff 로 부호 확장을 막아 0~255 로 다룬다.
public class ByteUtils {

    // b[start..start+len) 를 부호 없는 정수로. (상수 풀 length: len=2)
    public static int bytes2Int(byte[] b, int start, int len) {
        int sum = 0;
        for (int i = 0; i < len; i++) {
            int n = b[start + i] & 0xff;   // 부호 확장 방지
            int shift = (len - 1 - i) * 8; // 상위 바이트(앞쪽)일수록 더 왼쪽으로
            sum |= n << shift;
        }
        return sum;
    }

    // 정수를 len 바이트 배열로. (새 length 필드 쓰기)
    public static byte[] int2Bytes(int value, int len) {
        byte[] b = new byte[len];
        for (int i = 0; i < len; i++) {
            b[len - 1 - i] = (byte) ((value >> (8 * i)) & 0xff);
        }
        return b;
    }

    // originalBytes 의 [offset, offset+len) 구간을 replaceBytes 로 교체한 새 배열을 반환.
    // 교체 전후 길이가 달라도 됨(여기서 16바이트→10바이트로 줄이는 데 사용).
    public static byte[] bytesReplace(byte[] originalBytes, int offset, int len, byte[] replaceBytes) {
        byte[] newBytes = new byte[originalBytes.length + (replaceBytes.length - len)];
        System.arraycopy(originalBytes, 0, newBytes, 0, offset);                       // 앞부분 그대로
        System.arraycopy(replaceBytes, 0, newBytes, offset, replaceBytes.length);      // 교체 구간
        System.arraycopy(originalBytes, offset + len,                                  // 뒷부분 당겨 붙임
                newBytes, offset + replaceBytes.length,
                originalBytes.length - offset - len);
        return newBytes;
    }
}
