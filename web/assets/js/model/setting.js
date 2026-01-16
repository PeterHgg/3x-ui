class AllSetting {

    constructor(data) {
        this.webListen = "";
        this.webDomain = "";
        this.webPort = 2053;
        this.webCertFile = "";
        this.webKeyFile = "";
        this.webBasePath = "/";
        this.sessionMaxAge = 360;
        this.pageSize = 25;
        this.expireDiff = 0;
        this.trafficDiff = 0;
        this.remarkModel = "-ieo";
        this.datepicker = "gregorian";
        this.tgBotEnable = false;
        this.tgBotToken = "";
        this.tgBotProxy = "";
        this.tgBotAPIServer = "";
        this.tgBotChatId = "";
        this.tgRunTime = "@daily";
        this.tgBotBackup = false;
        this.tgBotLoginNotify = true;
        this.tgCpu = 80;
        this.tgLang = "en-US";
        this.twoFactorEnable = false;
        this.twoFactorToken = "";
        this.xrayTemplateConfig = "";
        this.subEnable = true;
        this.subJsonEnable = false;
        this.subTitle = "";
        this.subListen = "";
        this.subPort = 2096;
        this.subPath = "/sub/";
        this.subJsonPath = "/json/";
        this.subDomain = "";
        this.externalTrafficInformEnable = false;
        this.externalTrafficInformURI = "";
        this.subCertFile = "";
        this.subKeyFile = "";
        this.subUpdates = 12;
        this.subEncrypt = true;
        this.subShowInfo = true;
        this.subURI = "";
        this.subJsonURI = "";
        this.subJsonFragment = "";
        this.subJsonNoises = "";
        this.subJsonMux = "";
        this.subJsonRules = "";

        // Clash settings
        this.clashDomain = "";
        this.clashSubDomain = "";
        this.clashPrefix = "";
        this.clashCount = 0;
        this.clashNoPort = false;
        this.clashCustomRules = `DOMAIN-SUFFIX,szbdyd.com,REJECT
DOMAIN-SUFFIX,mcdn.bilivideo.com,REJECT
DOMAIN-SUFFIX,mcdn.bilivideo.cn,REJECT
DOMAIN-SUFFIX,edge.mountaintoys.cn,REJECT
DOMAIN-SUFFIX,scaleway.com,DIRECT
DOMAIN-SUFFIX,linux.do,🚀 手动切换
DOMAIN-SUFFIX,epicgames.com,DIRECT
DOMAIN-SUFFIX,epicgames.dev,DIRECT
DOMAIN-SUFFIX,epicgames.net,DIRECT
DOMAIN-SUFFIX,unrealengine.com,DIRECT
DOMAIN,steamcdn-a.akamaihd.net,DIRECT
DOMAIN-SUFFIX,cm.steampowered.com,DIRECT
DOMAIN-SUFFIX,steamserver.net,DIRECT
DOMAIN,steam-chat.com,🚀 手动切换
DOMAIN-SUFFIX,steamstatic.com,🚀 手动切换
DOMAIN,api.steampowered.com,🚀 手动切换
DOMAIN,store.steampowered.com,🚀 手动切换
DOMAIN-SUFFIX,steamcommunity.com,🚀 手动切换
DOMAIN-SUFFIX,steamgames.com,DIRECT
DOMAIN-SUFFIX,steamusercontent.com,DIRECT
DOMAIN-SUFFIX,steamcontent.com,🚀 手动切换
DOMAIN-SUFFIX,steamstatic.com,DIRECT
DOMAIN-SUFFIX,steamcdn-a.akamaihd.net,DIRECT
DOMAIN-SUFFIX,steamstat.us,DIRECT`;

        this.timeLocation = "Local";

        // LDAP settings
        this.ldapEnable = false;
        this.ldapHost = "";
        this.ldapPort = 389;
        this.ldapUseTLS = false;
        this.ldapBindDN = "";
        this.ldapPassword = "";
        this.ldapBaseDN = "";
        this.ldapUserFilter = "(objectClass=person)";
        this.ldapUserAttr = "mail";
        this.ldapVlessField = "vless_enabled";
        this.ldapSyncCron = "@every 1m";
        this.ldapFlagField = "";
        this.ldapTruthyValues = "true,1,yes,on";
        this.ldapInvertFlag = false;
        this.ldapInboundTags = "";
        this.ldapAutoCreate = false;
        this.ldapAutoDelete = false;
        this.ldapDefaultTotalGB = 0;
        this.ldapDefaultExpiryDays = 0;
        this.ldapDefaultLimitIP = 0;

        if (data == null) {
            return
        }
        ObjectUtil.cloneProps(this, data);
    }

    equals(other) {
        return ObjectUtil.equals(this, other);
    }
}