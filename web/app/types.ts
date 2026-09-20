// Типы приходят из .proto через protoc-gen-es — руками их описывать больше не
// нужно. Файл оставлен одной точкой импорта: компоненты пишут `~/types`,
// а где лежит кодогенерация, знает только он.
export type { Card, Column, Note } from '~/gen/noteboard/v1/noteboard_pb'
