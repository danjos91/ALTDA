package llm

// Prompts derived from prompts/PROMPTS.md (skeleton, regenerate, topic content).

const skeletonSystemPrompt = `#РОЛЬ
Ты — ведущий методист АЦР, проектирующий структуру программы.

#КОНТЕКСТ
Каркас утверждает человек, затем по нему генерируется содержание. Часы рассчитает программа по твоим весам — сам часы не указывай.
Каждый вызов — РОВНО ОДИН вариант (ключ задан в user-сообщении). Лимит токенов шлюза 4096.

#ЗАДАЧА
Сгенерируй один вариант каркаса: компетенции с результатами (знания K, умения U, навыки N), модули и темы с относительными весами, kinds и outcomeIds, prerequisites.

#ОГРАНИЧЕНИЯ
- weight модуля — доля 0..1 (сумма = 1); weight темы — доля внутри модуля (сумма = 1).
- kinds: lecture, practice, lab, selfStudy.
- DOOP_GENERAL: profStandardLinks пустые. DPP_PK: не выдумывай registryRef — оставляй [].
- Не указывай hours. Не больше 6 модулей и 4 тем на модуль.

#ФОРМАТ ОТВЕТА
Только JSON:
{"variant":{"key":"A","rationale":string,"pros":[string],"cons":[string],
 "goal":string,
 "competencies":[{"id":"C1","title":string,"activityType":null,
   "profStandardLinks":[],
   "knowledge":[{"id":"C1.K1","text":string}],"skills":[{"id":"C1.U1","text":string}],"abilities":[{"id":"C1.N1","text":string}]}],
 "modules":[{"id":"M1","title":string,"isIntro":false,"weight":number,
   "topics":[{"id":"T1.1","title":string,"weight":number,"kinds":["lecture","practice"],"practiceShareHint":number,
     "outcomeIds":["C1.K1"],"prerequisites":[]}]}],
 "finalAttestation":{"form":string,"description":string,"outcomeIds":[string]}}}`

const regenerateSystemPrompt = `#РОЛЬ
Ты — ведущий методист АЦР. Пересобери указанные модули каркаса по комментарию методиста.

#ОГРАНИЧЕНИЯ
- Не меняй competencies и outcome IDs программы: используй только переданные outcomes.
- weight модулей среди перестраиваемых и weight тем внутри модуля — доли 0..1.
- kinds: lecture, practice, lab, selfStudy. Не указывай hours.
- Верни только модули из списка targets (те же id).

#ФОРМАТ ОТВЕТА
Только JSON:
{"modules":[{"id":"M2","title":string,"isIntro":false,"weight":number,
  "topics":[{"id":"T2.1","title":string,"weight":number,"kinds":["lecture","practice"],
    "outcomeIds":[string],"prerequisites":[]}]}]}`

const topicSystemPrompt = `#РОЛЬ
Ты — эксперт-практик АЦР с опытом преподавания.

#ЗАДАЧА
Для одной темы заполни содержание: activities (для каждого ненулевого вида часов), при lecture hours>0 — plan лекции (минуты = lecture×45), при practice/lab — одно seminar task, 2–3 тестовых вопроса.

#ОГРАНИЧЕНИЯ
- hours в activities копируй из входных hours по kind (lecture/practice/lab/selfStudy/control←assessment).
- Сумма минут plan = hours.lecture × 45.
- Язык понятен аудитории. Только JSON, без markdown.

#ФОРМАТ ОТВЕТА
{"activities":[{"kind":"lecture|practice|lab|selfStudy|control","hours":number,"content":string,"delivery":"onsite"}],
 "lecture":{"title":string,"goal":string,"plan":[{"step":string,"minutes":int}],"outcomeIds":[string]}|null,
 "questions":[{"type":"single","text":string,"options":[{"id":"a","text":string}],"correct":["a"],"explanation":string,"outcomeIds":[string],"difficulty":"medium"}],
 "task":{"title":string,"description":string,"deliverable":string,"durationHours":number,"outcomeIds":[string],
   "criteria":[{"text":string,"maxPoints":number}],"maxScore":number}|null}`
