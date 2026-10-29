// 클래스 byte[] 의 상수 풀을 훑어, CONSTANT_Utf8 항목 중 oldStr 과 같은 것을 newStr 로 치환한다.
// 핵심 디테일: Utf8 항목은 [tag(1)][length(2)][bytes(length)] 라, 문자열을 바꾸면
//   length 필드(2바이트)도 새 길이로 고치고, 줄어든 만큼 뒤 바이트를 당겨야 한다(ByteUtils.bytesReplace).
//
// 상수 풀 구조(JVM Spec §4.4):
//   - 클래스 파일 8바이트(magic 4 + minor 2 + major 2) 뒤 offset 8 에 constant_pool_count(2바이트).
//   - 그 뒤부터 (count-1) 개의 상수 풀 항목. 각 항목은 tag(1바이트)로 종류가 갈리고 크기가 다르다.
//   - 이 실습은 Utf8(tag=1) 만 관심. 나머지 tag 는 정해진 고정 크기만큼 건너뛴다.
public class ClassModifier {

    private static final int CONSTANT_POOL_COUNT_INDEX = 8;   // constant_pool_count 위치
    private static final int CONSTANT_Utf8_info = 1;          // Utf8 태그
    // tag → 그 항목이 tag 이후 차지하는 바이트 수 (Utf8 은 가변이라 별도 처리, 여기엔 0)
    private static final int[] CONSTANT_ITEM_LENGTH = {
        -1, -1, -1, 4, 4, 8, 8, 2, 2, 4, 4, 4, 4, -1, -1, 3, 2, -1, 4
        // index = tag. 3=Integer,4=Float(4) / 5=Long,6=Double(8) / 7=Class,8=String(2)
        // 9=Fieldref..12=NameAndType(4) / 15=MethodHandle(3) / 16=MethodType(2) / 18=InvokeDynamic(4)
    };
    private static final int u1 = 1, u2 = 2;

    private byte[] classByte;

    public ClassModifier(byte[] classByte) {
        this.classByte = classByte;
    }

    // 상수 풀의 Utf8 항목 중 oldStr 을 newStr 로 치환. 치환된 byte[] 반환.
    public byte[] modifyUTF8Constant(String oldStr, String newStr) {
        int cpc = getConstantPoolCount();
        int offset = CONSTANT_POOL_COUNT_INDEX + u2;   // 첫 항목 시작
        for (int i = 0; i < cpc - 1; i++) {
            int tag = ByteUtils.bytes2Int(classByte, offset, u1);
            if (tag == CONSTANT_Utf8_info) {
                int lenStart = offset + u1;
                int len = ByteUtils.bytes2Int(classByte, lenStart, u2);
                offset += (u1 + u2);                    // tag + length 만큼 전진 → 실제 바이트 시작
                String str = new String(classByte, offset, len, java.nio.charset.StandardCharsets.UTF_8);
                if (str.equals(oldStr)) {
                    // 1) 실제 문자열 바이트 교체
                    byte[] newBytes = newStr.getBytes(java.nio.charset.StandardCharsets.UTF_8);
                    classByte = ByteUtils.bytesReplace(classByte, offset, len, newBytes);
                    // 2) length 필드(앞 2바이트)도 새 길이로 보정
                    byte[] newLen = ByteUtils.int2Bytes(newBytes.length, u2);
                    classByte = ByteUtils.bytesReplace(classByte, lenStart, u2, newLen);
                    return classByte;   // 첫 일치만 치환(이 실습은 System 한 항목)
                } else {
                    offset += len;       // 이 Utf8 건너뛰기
                }
            } else {
                if (tag == 5 || tag == 6) i++;   // Long/Double 은 상수 풀 슬롯 2칸 차지
                offset += (u1 + CONSTANT_ITEM_LENGTH[tag]);
            }
        }
        return classByte;
    }

    public int getConstantPoolCount() {
        return ByteUtils.bytes2Int(classByte, CONSTANT_POOL_COUNT_INDEX, u2);
    }
}
