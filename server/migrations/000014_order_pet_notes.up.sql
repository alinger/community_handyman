-- 遛宠流程图 P0：宠物备注独立成列。
-- 之前只有通用 remark，宠物的类型/名字/特殊情况与「门禁怎么进」混在一起，
-- 服务方接单时无法快速抓住照顾要点。
ALTER TABLE orders
    ADD COLUMN pet_notes VARCHAR(200) NOT NULL DEFAULT '' COMMENT '宠物备注：类型、名字、情况等，选填' AFTER patient_name;
