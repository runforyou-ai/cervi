/** 企业团队与成员分配调用与归一化。 */
import {
  AddTeamMembers,
  CreateTeam,
  DeleteTeam,
  ListMemberOptions,
  ListTeamMemberCandidates,
  ListTeamMembers,
  ListTeams,
  RemoveTeamMembers,
  UpdateTeam,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import type {
  MemberOption,
  MemberOptionList,
  MemberOptionListInput,
  TeamListInput,
  TeamMember,
  TeamMemberCandidate,
  TeamMemberCandidateInput,
  TeamMemberCandidateList,
  TeamMemberList,
  TeamMemberListInput,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind } from "@/api/client"
import { asList } from "@/api/normalize"

export type TeamListQuery = Partial<TeamListInput>

export type TeamMemberCandidateQuery = Partial<TeamMemberCandidateInput>

export type TeamMemberListQuery = Partial<TeamMemberListInput>

export type MemberOptionListQuery = Partial<MemberOptionListInput>

export type MemberOptionListData = Omit<MemberOptionList, "members"> & {
  members: MemberOption[]
}

export type TeamMemberListData = Omit<TeamMemberList, "members"> & {
  members: TeamMember[]
}

export type TeamMemberCandidateListData = Omit<
  TeamMemberCandidateList,
  "members"
> & {
  members: TeamMemberCandidate[]
}

const listTeamsBound = bind(ListTeams)
const listMemberOptionsBound = bind(ListMemberOptions)
const listTeamMemberCandidatesBound = bind(ListTeamMemberCandidates)
const listTeamMembersBound = bind(ListTeamMembers)

/** 创建企业团队。 */
export const createTeam = bind(CreateTeam)

/** 修改企业团队。 */
export const updateTeam = bind(UpdateTeam)

/** 删除企业团队。 */
export const deleteTeam = bind(DeleteTeam)

/** 将企业身份批量加入团队。 */
export const addTeamMembers = bind(AddTeamMembers)

/** 将企业身份批量移出团队。 */
export const removeTeamMembers = bind(RemoveTeamMembers)

/** 读取尚未加入团队的企业成员。 */
export function listTeamMemberCandidates(
  teamId: string,
  query: TeamMemberCandidateQuery = {},
  signal?: AbortSignal,
): Promise<TeamMemberCandidateListData> {
  return listTeamMemberCandidatesBound(
    teamId,
    {
      query: query.query ?? "",
      page: query.page ?? 1,
      pageSize: query.pageSize ?? 50,
    },
    signal,
  ).then((output) => ({
    ...output,
    members: asList(output.members),
  }))
}

/** 读取团队成员列表。 */
export function listTeamMembers(
  teamId: string,
  query: TeamMemberListQuery,
  signal?: AbortSignal,
) {
  return listTeamMembersBound(
    teamId,
    {
      query: query.query ?? "",
      workStatus: query.workStatus ?? null,
      page: query.page ?? 1,
      pageSize: query.pageSize ?? 50,
    },
    signal,
  ).then((output) => ({
    ...output,
    members: asList(output.members),
  }))
}

/** 读取企业团队列表。 */
export function listTeams(query: TeamListQuery = {}, signal?: AbortSignal) {
  return listTeamsBound(
    {
      query: query.query ?? "",
      page: query.page ?? 1,
      pageSize: query.pageSize ?? 50,
    },
    signal,
  ).then((output) => ({ ...output, teams: asList(output.teams) }))
}

/** 读取可分配的企业成员和 AI 员工。 */
export function listMemberOptions(
  query: MemberOptionListQuery = {},
  signal?: AbortSignal,
): Promise<MemberOptionListData> {
  return listMemberOptionsBound(
    {
      query: query.query ?? "",
      page: query.page ?? 1,
      pageSize: query.pageSize ?? 50,
    },
    signal,
  ).then((output) => ({ ...output, members: asList(output.members) }))
}
