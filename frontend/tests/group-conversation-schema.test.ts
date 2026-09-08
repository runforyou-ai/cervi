/** 验证跨端群聊表单共享的业务边界和本地化校验。 */
import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import { stripTypeScriptTypes } from "node:module"
import { test } from "node:test"
import { runInNewContext } from "node:vm"
import { z } from "zod"

const source =
  stripTypeScriptTypes(
    readFileSync(
      new URL("../src/features/inbox/group-conversation-schema.ts", import.meta.url),
      "utf8",
    ),
  )
    .replace(/import.*from "zod"/, "")
    .replace(/export (const|function)/g, "$1") +
  "\nexports.profile = createGroupProfileSchema; exports.create = createGroupConversationSchema"
const schemas: Record<string, any> = {}
runInNewContext(source, { z, exports: schemas })
const translate = (key: string) => `translated:${key}`

test("群名称去除首尾空格，空名称与超长资料返回业务翻译", () => {
  const schema = schemas.profile(translate)
  assert.equal(
    schema.parse({ title: "  项目群  ", description: "  说明  " }).title,
    "项目群",
  )
  for (const [title, description, key] of [
    ["   ", "", "groupTitleRequired"],
    ["名".repeat(101), "", "groupTitleTooLong"],
    ["项目群", "描".repeat(501), "groupDescriptionTooLong"],
  ]) {
    const result = schema.safeParse({ title, description })
    assert.equal(result.success, false)
    assert.equal(result.error.issues[0].message, translate(key))
  }
})

test("成员编号和成员对象使用同一初始人数限制，并保留各端表单值", () => {
  for (const item of [
    z.string(),
    z.object({ id: z.string(), displayName: z.string() }),
  ]) {
    const schema = schemas.create(translate, item)
    const members = Array.from({ length: 99 }, (_, index) =>
      item instanceof z.ZodString
        ? String(index)
        : { id: String(index), displayName: `成员${index}` },
    )
    const input = { title: "项目群", description: "", members }
    assert.deepEqual(schema.parse(input).members, members)
    assert.equal(
      schema.safeParse({ ...input, members: [] }).error.issues[0].message,
      translate("groupMembersRequired"),
    )
    assert.equal(
      schema.safeParse({ ...input, members: [...members, members[0]] }).error.issues[0]
        .message,
      translate("groupMembersTooMany"),
    )
  }
})
