---
title: "Модуль flant-integration"
---

## Описание

Модуль выполняет функции по интеграции с различными сервисами Flant:

* Устанавливает в кластер madison-proxy в качестве alertmanager для prometheus. Регистрируется в Madison.
* Отправляет детальную статистику по кластерам для расчета стоимости обслуживания кластера.
* Доставка логов Deckhouse до хранилища логов Flant для облегчения процесса отладки.

### Отключение автоматической регистрации в Madison

Делается за счет отключения модуля `flantIntergation`.

**Важно!** Есть два случая, когда необходимо **обязательно** отключить автоматическую регистрацию в Madison:

1. В **тестовом кластере**, который вы развернули для своих экспериментов (или каких-то экспериментов в команде)
   нужно **обязательно отключать алерты**. Это правило НЕ относится к dev-кластерам клиентов, в которых алерты нам
   обязательно нужны.
2. В любых **кластерах снятых с поддержки** (например, когда мы расстаемся с клиентом).

### Расчет стоимости

Для каждой NodeGroup, за исключением выделенных мастеров, автоматически вычисляется тип биллинга. Существуют следующие
типы биллинга узлов:

* Ephemeral - если узел относится к NodeGroup с типом Cloud, то она автоматически относится к Ephemeral.
* VM - данный тип проставляется автоматически, если для узла удалось определить тип виртуализации с помощью команды
  [virt-what](https://people.redhat.com/~rjones/virt-what/).
* Hard - все остальные узлы автоматически относятся к данному типу.
* Special - данный тип необходимо вручную проставлять на NodeGroup, сюда относятся выделенные узлы, которые нельзя
  "потерять".

В случае, если в кластере есть узлы с типом биллинга Special или автоматическое определение сработало некорректно, 
то вы всегда можете вручную установить корректный тип биллинга.

Для установки типа биллинга на узлах рекомендуется устанавливать аннотацию на NodeGroup, к которой относится узел:

```
kubectl patch ng worker --patch '{"spec":{"nodeTemplate":{"annotations":{"pricing.flant.com/nodeType":"Special"}}}}' --type=merge
```

Если в рамках одной NodeGroup есть узлы с разными типами биллинга, то можно навесить аннотацию отдельно на каждый объект Node:

```
kubectl annotate node test pricing.flant.com/nodeType=Special
```

#### Определение статусов terraform-стейтов

Модуль опирается на метрики экспортируемые компонентом `terraform-exporter`. В них содержатся статусы соответствия
ресурсов в облаке/кластере с заданными в конфигурациях `*-cluster-configuration`.

##### Исходные метрики `terraform-exporter` и их статусы.

1. `candi_converge_cluster_status` соответствие конфигурации базовой инфраструктуры:
    - `error` - ошибка обработки, подробности смотреть в логе экспортера.
    - `destructively_changed` - `terraform plan` предполагает изменение объектов в облаке с удалением какого-либо из них.
    - `changed` - `terraform plan` предполагает изменение объектов в облаке без их удаления.
    - `ok`
1. `candi_converge_node_status` - соответствие конфигурации отдельных Node:
    - `error` - ошибка обработки, подробности смотреть в логе экспортера.
    - `destructively_changed` - `terraform plan` предполагает изменение объектов в облаке с удалением какого-либо из них.
    - `abandoned` - в кластере лишняя Node.
    - `absent` - в кластере не хватает Node.
    - `changed` - `terraform plan` предполагает изменение объектов в облаке без их удаления.
    - `ok`
1. `candi_converge_node_template_status` - соответствие `nodeTemplate` для `master` и `terranode` NodeGroup:
    - `absent` - NodeGroup отсутствует в кластере.
    - `changed` - параметры `nodeTemplate` расходятся.
    - `ok`

##### Конечные метрики модуля `flant-integration` и механизм их получения.

> Если модуль `terraform-manager` выключен в кластере — статус во всех метриках будет `none`. Данный статус следует трактовать как: стейта в кластере нет, но и не должно быть.

1. Статус кластера (базовой инфраструктуры):
    - Используется значение метрики `candi_converge_cluster_status`.
    - В случае отсутствия метрики - `missing`.
1. Статус `master` NodeGroup:
    - Берется "худший" статус из метрик `candi_converge_node_status` и `candi_converge_node_template_status` для `ng/master`.
    - В случае отсутствия обоих метрик - `missing`.
1. Отдельный статус по каждой `terranode` NodeGroup:
    - Берется "худший" статус из метрик `candi_converge_node_status` и `candi_converge_node_template_status` для `ng/<nodeGroups[].name>`.
1. Суммарный статус для всех `terranode` NodeGroup:
    - Берется "худший" статус из статусов, полученных для всех `terranode` NodeGroup.

> Статус `missing` так же будет фигурировать в конечных метриках, если `terraform-exporter` начнёт отдавать в своих метриках не описанные в модуле статусы. Иными словами статус `missing` это еще и некоего рода `fallback`-статус для ситуации, когда что-то пошло не так с определением "худшего" статуса.

##### Как определяется "худший" статус.

Мы считаем "худший" с точки зрения возможности автоматического применения существующих изменений.

Выбирается он по приоритету из следующей таблицы известных статусов:

| Статус                | Описание                                                                                  |
| --------------------- | ----------------------------------------------------------------------------------------- |
| error                 | Ошибка обработки стейта `terraform-exporter`'ом, подробности в его логе.                  |
| destructively_changed | `terraform plan` предполагает изменение объектов в облаке с удалением какого-либо из них. |
| abandoned             | В кластере лишняя Node.                                                                   |
| absent                | В кластере не хватает Node или NodeGroup.                                                 |
| changed               | `terraform plan` предполагает изменение объектов в облаке без их удаления.                |
| ok                    | Расхождений не обнаружено.                                                                |
